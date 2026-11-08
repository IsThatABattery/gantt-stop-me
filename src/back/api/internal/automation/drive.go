package automation

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/runner"
)

// execution is a claimed workflow execution, as drive carries it out.
type execution struct {
	id, runID int64
	project   projects.Project
	head      string // the commit under test
	spec      runner.Spec
	timeout   time.Duration // the workflow's, from the claim on
	claimed   time.Time
	attempts  int           // claims so far, this one included
	token     string        // the automation owner's GitHub credential
	log       string        // the log tail: build and start output, then each script's, the last 64 KB of it
	issue     *github.Issue // the task's issue, read before the first LLM check
	browser   *browser      // the browser's MCP session, opened before the first LLM check when the app runs
}

// result is one test of an execution and, once it ends, how it went.
type result struct {
	id         int64
	position   int
	text       string // the test's line and block
	kind       string // "script" or "llm"
	command    string // a script's
	status     string // "pending" until it ends, then "passed", "failed", "manual" or "error"
	detail     string // the exit code and output, or why
	transcript []byte // an LLM check's tool calls as JSON, nil for none
	screenshot []byte // an LLM check's PNG, nil for none
}

var (
	// errCancelled ends an execution's context when it or its test run is cancelled.
	errCancelled = errors.New("automation: the workflow execution was cancelled")
	// errTimedOut ends an execution's context when its workflow's timeout passes.
	errTimedOut = errors.New("automation: the workflow timed out")
)

// interruptedTwice is the runner error of the tests of an execution the runner or the API dropped twice.
const interruptedTwice = "The runner was interrupted twice."

// maxLog is the most of an execution's log the API keeps: its tail.
const maxLog = 64 << 10

// drive carries out a claimed workflow execution, as Running tests says, within its workflow's timeout, and stops
// within a couple of seconds of it or its run being cancelled. Whatever happened, it then removes the execution from
// the runner and records how it ended.
func (a *Automation) drive(ctx context.Context, e *execution) {
	xctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	xctx, stop := context.WithDeadlineCause(xctx, e.claimed.Add(e.timeout), errTimedOut)
	defer stop()
	unwatch := a.watch(xctx, e.id, cancel)
	err := a.carryOut(xctx, e)
	unwatch()
	// Left running, e would hold its place in the queue until the API restarts, so a failure to record its end is
	// tried again every second until the worker stops, after which Recover mends it.
	settle := func() {
		for {
			sErr := a.settle(ctx, xctx, e, err)
			if sErr == nil {
				return
			}
			log.Printf("end workflow execution %d: %v", e.id, sErr)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
	// The end is recorded after Delete, so the queue starts nothing beside an execution still being removed. But
	// shutting down, nothing claims any more: recorded first, a kill on the way down costs no attempt, and Recover's
	// DeleteAll removes what Delete did not.
	shutdown := ctx.Err() != nil
	if shutdown {
		settle()
	}
	// A fresh context: the execution's may have ended, and its containers go either way.
	dctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	if err := a.Runner.Delete(dctx, e.id); err != nil {
		log.Printf("remove workflow execution %d from the runner: %v", e.id, err)
	}
	done()
	if !shutdown {
		settle()
	}
}

// watch cancels ctx with errCancelled once execution id or its test run is no longer running, checking every second,
// until the returned stop is called.
func (a *Automation) watch(ctx context.Context, id int64, cancel context.CancelCauseFunc) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			var running bool
			err := a.DB.QueryRow(ctx, `
				select e.status = 'running' and r.status in ('queued', 'running')
				from workflow_executions e join test_runs r on r.id = e.run_id where e.id = $1`, id).Scan(&running)
			if errors.Is(err, pgx.ErrNoRows) || err == nil && !running { // gone with its task, or cancelled
				cancel(errCancelled)
				return
			}
		}
	})
	return func() {
		close(done)
		wg.Wait()
	}
}

// carryOut prepares execution e from the source at its head, then runs its pending scripts and then its pending LLM
// checks, one at a time in issue order, saving each result as it ends. It returns what stopped it early: ctx ending,
// GitHub failing, marked gitHubFailure, the runner failing, or the runner restarting or out of reach, marked
// interrupted or unreachable.
func (a *Automation) carryOut(ctx context.Context, e *execution) error {
	rows, _ := a.DB.Query(ctx, `
		select id, position, text, kind, coalesce(command, ''), status from test_results
		where execution_id = $1 and status = 'pending' order by position`, e.id)
	results, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (result, error) {
		var r result
		return r, row.Scan(&r.id, &r.position, &r.text, &r.kind, &r.command, &r.status)
	})
	if err != nil {
		return err
	}
	source, err := a.GitHub.Tarball(ctx, e.token, e.project.RepoID, e.head)
	if err != nil {
		return gitHubFailure{fmt.Errorf("download the source from GitHub: %w", err)}
	}
	out, err := a.Runner.Prepare(ctx, e.id, e.spec, source)
	source.Close()
	if se := (*runner.SourceError)(nil); errors.As(err, &se) { // the download broke off partway
		return gitHubFailure{fmt.Errorf("download the source from GitHub: %w", err)}
	}
	if f := (*runner.Failure)(nil); errors.As(err, &f) {
		out = f.Log
	}
	e.logs(out)
	if err != nil {
		return fromRunner(err)
	}
	for _, kind := range []string{"script", "llm"} {
		for _, r := range results {
			if r.kind != kind {
				continue
			}
			if kind == "script" {
				r, err = a.script(ctx, e, r)
			} else {
				r, err = a.check(ctx, e, r)
			}
			if err == nil {
				err = a.save(ctx, e, r)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// script runs a script test as one command in a fresh container: exit 0 passes. The detail holds the exit code and
// the output, which also goes on the log tail. The runner refusing the command is a runner error for this test only.
func (a *Automation) script(ctx context.Context, e *execution, r result) (result, error) {
	code, out, err := a.Runner.Command(ctx, e.id, r.command)
	if re := (*runner.Error)(nil); errors.As(err, &re) && re.Status != http.StatusNotFound {
		r.status, r.detail = "error", "The runner could not run the command: "+re.Message
		return r, nil
	}
	if err != nil {
		return r, fromRunner(err)
	}
	e.logs("$ " + r.command + "\n" + out)
	r.status = "failed"
	if code == 0 {
		r.status = "passed"
	}
	r.detail = fmt.Sprintf("Exit code %d.", code)
	if out != "" {
		r.detail += "\n" + out
	}
	return r, nil
}

// save stores result r of e, which just ended, with e's log tail, unless a cancel ended r first. It goes on when ctx
// has ended, since the result is real.
func (a *Automation) save(ctx context.Context, e *execution, r result) error {
	_, err := a.DB.Exec(context.WithoutCancel(ctx), `
		with tail as (update workflow_executions set log_tail = $7 where id = $6)
		update test_results set status = $2, detail = $3, transcript = $4, screenshot = $5 where id = $1 and status = 'pending'`,
		r.id, r.status, clean(r.detail), r.transcript, r.screenshot, e.id, e.log)
	return err
}

// interrupted marks an error of the runner's that means it restarted mid-execution: no answer at all, or a 404 for
// an execution it no longer has.
type interrupted struct{ error }

func (i interrupted) Unwrap() error { return i.error }

// unreachable marks an error of the runner's that means the request never reached it, such as while it restarts: it
// was not interrupted by it.
type unreachable struct{ error }

func (u unreachable) Unwrap() error { return u.error }

// gitHubFailure marks an error of a GitHub call an execution made, such as its source's download.
type gitHubFailure struct{ error }

func (g gitHubFailure) Unwrap() error { return g.error }

// fromRunner marks err, from a runner call, unreachable when it could not connect, or interrupted unless it is the
// runner's own answer. carryOut has taken the source failing out already.
func fromRunner(err error) error {
	var f *runner.Failure
	var re *runner.Error
	if err == nil || errors.As(err, &f) || errors.As(err, &re) && re.Status != http.StatusNotFound {
		return err
	}
	if op := (*net.OpError)(nil); errors.As(err, &op) && op.Op == "dial" {
		return unreachable{err}
	}
	return interrupted{err}
}

// settle records how execution e ended after carryOut stopped with err. xctx is e's own context and ctx the
// worker's. Its run being cancelled skips the unfinished tests, and the timeout fails them as a runner error. The API
// shutting down, the runner being unreachable, GitHub failing for now, or GitHub refusing the owner's credential,
// which pauses the project, puts e back on the queue without counting the attempt; the runner restarting puts it back
// once, and the second time is a runner error. Any other error is a runner error for every unfinished test.
// shortcut: an unreachable runner is tried again every pass, each try downloading the source from GitHub; back off
// if runners stay down for long.
func (a *Automation) settle(ctx, xctx context.Context, e *execution, err error) error {
	db := context.WithoutCancel(xctx)
	// Checked here as well as by the watch, which may not have looked since a cancel.
	var live bool
	if qErr := a.DB.QueryRow(db, `select status in ('queued', 'running') from test_runs where id = $1`, e.runID).
		Scan(&live); qErr != nil && !errors.Is(qErr, pgx.ErrNoRows) { // no rows: gone with its task
		return qErr
	}
	switch cause := context.Cause(xctx); {
	case !live, errors.Is(cause, errCancelled):
		return a.end(db, e, "skipped", "")
	case err == nil:
		return a.end(db, e, "", "")
	case errors.Is(cause, errTimedOut):
		return a.end(db, e, "error", "The workflow timed out after "+short(e.timeout)+".")
	case ctx.Err() != nil, errors.As(err, new(unreachable)):
		return a.requeue(db, e, false)
	case rejected(err):
		a.PauseIfRejected(db, e.project.ID, e.token, err)
		return a.requeue(db, e, false)
	case errors.As(err, new(gitHubFailure)) && !lasting(err):
		return a.requeue(db, e, false)
	case errors.As(err, new(interrupted)) && e.attempts < 2:
		return a.requeue(db, e, true)
	case errors.As(err, new(interrupted)):
		return a.end(db, e, "error", interruptedTwice)
	}
	return a.end(db, e, "error", reason(err))
}

// end ends execution e: it gives its unfinished tests status and detail, when status is set, and saves its log tail.
// Then e takes its status from its results, or is cancelled when status is "skipped", unless a cancel ended it first.
func (a *Automation) end(ctx context.Context, e *execution, status, detail string) error {
	set := settled
	if status == "skipped" {
		set = `status = 'cancelled', finished_at = now()`
	}
	return pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if status != "" {
			if _, err := tx.Exec(ctx, `update test_results set status = $2, detail = $3 where execution_id = $1 and status = 'pending'`,
				e.id, status, clean(detail)); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `update workflow_executions set log_tail = $2 where id = $1`, e.id, e.log); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `update workflow_executions e set `+set+` where e.id = $1 and e.status = 'running'`, e.id)
		return err
	})
}

// requeue puts execution e back on the queue, its unfinished tests still pending, unless a cancel ended it first.
// counted keeps this claim as an attempt; a put-back that is not the runner's doing does not count.
func (a *Automation) requeue(ctx context.Context, e *execution, counted bool) error {
	uncount := 1
	if counted {
		uncount = 0
	}
	_, err := a.DB.Exec(ctx, `
		update workflow_executions set status = 'queued', started_at = null, attempts = attempts - $2, log_tail = $3
		where id = $1 and status = 'running'`, e.id, uncount, e.log)
	return err
}

// reason is the runner error of every unfinished test of an execution that err stopped.
func reason(err error) string {
	var f *runner.Failure
	var re *runner.Error
	switch {
	case errors.As(err, &f) && f.Stage == "build":
		return "The build failed. The execution's log says why."
	case errors.As(err, &f):
		return "The app or a sidecar did not start. The execution's log says why."
	case errors.As(err, &re):
		return "The runner refused the execution: " + re.Message
	}
	return "The execution failed: " + err.Error() + "."
}

// logs adds s to e's log tail, keeping the last maxLog bytes.
func (e *execution) logs(s string) {
	if s == "" {
		return
	}
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	e.log += s
	if len(e.log) > maxLog {
		e.log = e.log[len(e.log)-maxLog:]
	}
	e.log = clean(e.log)
}

// clean makes s fit a text column: valid UTF-8, which a cut can break, and no NUL, which Postgres refuses.
func clean(s string) string {
	return strings.ReplaceAll(strings.ToValidUTF8(s, ""), "\x00", "")
}

// short writes d as a workflow file would, such as 20m rather than 20m0s.
func short(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}
