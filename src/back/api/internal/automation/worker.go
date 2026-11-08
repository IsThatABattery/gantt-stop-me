package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
)

// outcome is the status a set of test results gives, first match wins: any failed is failed, anything else not
// passed or manual is a runner error, and the rest passed. Select it from test_results.
const outcome = `case when bool_or(status = 'failed') then 'failed'
	when bool_or(status not in ('passed', 'manual')) then 'error' else 'passed' end`

// settled sets workflow execution e's status from its test results, all ended, and its end.
const settled = `status = (select ` + outcome + ` from test_results where execution_id = e.id), finished_at = now()`

// Work runs the queue worker once, as RunWorker does every 2 seconds: it plans the queued test runs, claims the
// workflow executions the queue allows and drives them, and once those have ended, finishes the test runs that are
// over and posts their comments. Tests call it in place of RunWorker, which never runs there.
func (a *Automation) Work(ctx context.Context) {
	var drives sync.WaitGroup
	a.planAll(ctx)
	a.claimAll(ctx, &drives)
	drives.Wait()
	a.finishAll(ctx)
	a.commentAll(ctx)
}

// RunWorker runs the queue worker every 2 seconds until ctx ends, then waits for the executions it drives: ctx
// ending stops them, removes them from the runner and puts them back on the queue. It does nothing while RunnerURL
// is empty.
// shortcut: planning, finishing and comments run one after another in this loop, so a slow decisions call or GitHub
// stretches the 2 seconds for every project; give each its own goroutine if that bites.
func (a *Automation) RunWorker(ctx context.Context) {
	if a.RunnerURL == "" {
		return
	}
	var drives sync.WaitGroup
	defer drives.Wait()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		a.planAll(ctx)
		a.claimAll(ctx, &drives)
		a.finishAll(ctx)
		a.commentAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// claimAll claims workflow executions while the queue allows, and drives each one in a goroutine of drives. It checks
// the owner with OwnerToken before each starts: a paused project, or a check that failed for now, puts the execution
// back without counting the attempt.
func (a *Automation) claimAll(ctx context.Context, drives *sync.WaitGroup) {
	skip := []int64{} // put back this pass; not nil, which the claim would read as NULL and match nothing
	for ctx.Err() == nil {
		e, err := a.claim(ctx, skip)
		if err != nil || e == nil {
			report(ctx, "claim a workflow execution", err)
			return
		}
		e.token, err = a.OwnerToken(ctx, e.project.ID)
		switch {
		case errors.Is(err, ErrOff): // turning automation off cancelled it
		case err != nil:
			report(ctx, fmt.Sprintf("check the automation owner of project %d", e.project.ID), err)
			skip = append(skip, e.id)
			report(ctx, fmt.Sprintf("put workflow execution %d back", e.id), a.requeue(context.WithoutCancel(ctx), e, false))
		default:
			drives.Go(func() { a.drive(ctx, e) })
		}
	}
}

// claim takes the next queued workflow execution the queue allows, as Queue and restarts says: in id order across the
// install, while fewer than MaxConcurrentWorkflows run, never beside a running one of the same project and workflow,
// and never for a paused project, skipping those in skip. It marks the execution and its run running and counts the
// attempt. It returns nil when the queue allows none. An execution of a run that ended, or whose row a cancel holds,
// is not claimed: the run is locked with the execution, so a claim never waits on a cancel that waits on it.
func (a *Automation) claim(ctx context.Context, skip []int64) (*execution, error) {
	e := &execution{claimed: time.Now()}
	var spec []byte
	err := a.DB.QueryRow(ctx, `
		with next as (
			select e.id from workflow_executions e
			join test_runs r on r.id = e.run_id and r.status in ('queued', 'running')
			join project_automation pa on pa.project_id = e.project_id and pa.paused_reason is null
			where e.status = 'queued' and e.id <> all($2)
				and (select count(*) from workflow_executions where status = 'running') < $1
				and not exists (select from workflow_executions o
					where o.status = 'running' and o.project_id = e.project_id and o.workflow = e.workflow)
			order by e.id limit 1
			for update of e, r skip locked),
		claimed as (
			update workflow_executions e set status = 'running', started_at = now(), attempts = e.attempts + 1
			from next where e.id = next.id
			returning e.id, e.run_id, e.project_id, e.spec, e.timeout, e.attempts, e.log_tail),
		run as (update test_runs set status = 'running' where id = (select run_id from claimed) and status = 'queued')
		select c.id, c.run_id, p.id, p.github_repo_id, p.owner, p.name, coalesce(r.head_sha, ''), c.spec, c.timeout,
			c.attempts, c.log_tail
		from claimed c join test_runs r on r.id = c.run_id join projects p on p.id = c.project_id`,
		a.MaxConcurrentWorkflows, skip).
		Scan(&e.id, &e.runID, &e.project.ID, &e.project.RepoID, &e.project.Owner, &e.project.Name, &e.head, &spec,
			&e.timeout, &e.attempts, &e.log)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, json.Unmarshal(spec, &e.spec)
}

// Recover readies the queue when the API starts with automation on: it has the runner remove every execution,
// logging a failure and going on, then puts the workflow executions that were running back on the queue, once. One
// interrupted a second time ends instead, its unfinished tests a runner error, and the next pass finishes its run.
// One whose run was cancelled meanwhile, which the API stopped before it could remove it, is cancelled.
func (a *Automation) Recover(ctx context.Context) error {
	if a.RunnerURL == "" {
		return nil
	}
	dctx, cancel := context.WithTimeout(ctx, 2*time.Minute) // so a runner that hangs does not keep the API from starting
	err := a.Runner.DeleteAll(dctx)
	cancel()
	if err != nil {
		log.Printf("remove the runner's executions: %v", err)
	}
	return pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			update workflow_executions e set status = 'cancelled', finished_at = now() from test_runs r
			where r.id = e.run_id and e.status in ('queued', 'running') and r.status not in ('queued', 'running')`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			update test_results set status = 'error', detail = $1 where status = 'pending' and execution_id in
				(select id from workflow_executions where status = 'running' and attempts >= 2)`, interruptedTwice)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `update workflow_executions e set `+settled+` where e.status = 'running' and e.attempts >= 2`); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update workflow_executions set status = 'queued', started_at = null where status = 'running'`)
		return err
	})
}

// report logs err from the worker's step what, unless it is nil, the project being paused or off, or ctx ending.
func report(ctx context.Context, what string, err error) {
	if err != nil && ctx.Err() == nil && !errors.Is(err, ErrPaused) && !errors.Is(err, ErrOff) {
		log.Printf("%s: %v", what, err)
	}
}

// lasting reports whether err is GitHub refusing a call in a way no retry mends: a 4xx other than 401, which a new
// credential mends, and 429, which passes. Anything else, such as a 5xx or no answer, may pass on a retry.
func lasting(err error) bool {
	var ghErr *github.Error
	return errors.As(err, &ghErr) && ghErr.Status >= 400 && ghErr.Status < 500 &&
		ghErr.Status != http.StatusUnauthorized && ghErr.Status != http.StatusTooManyRequests
}
