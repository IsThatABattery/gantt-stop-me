package automation

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/board"
	"gantt-stop-me/api/internal/projects"
)

var (
	// runInProgress refuses moving a task out of Testing while its test run is queued or running.
	runInProgress = &board.Refusal{Code: "run_in_progress",
		Message: "This task's test run is still going. Cancel the run to move the task out of Testing."}
	// notTested refuses moving a task into User Check that has not passed on its pull request's current head.
	notTested = &board.Refusal{Code: "not_tested",
		Message: "A task enters User Check only once its latest test run passed on its pull request's current head."}
)

// Moving applies Moves while automated to a move of task taskID from column from to column to, in tx holding the
// project's lock, while the project's automation is on. A person's move, byHand, out of Testing while the task's
// latest test run is queued or running is refused with run_in_progress, and one into User Check with not_tested
// unless that run passed on its pull request's head as the record has it now. A person's move into Testing starts a
// test run for the task's linked pull request. Any move out of Testing clears needs_attention. It implements
// board.Rules.
func (a *Automation) Moving(ctx context.Context, tx pgx.Tx, p projects.Project, taskID int64, from, to string, byHand bool) error {
	var on bool
	err := tx.QueryRow(ctx, `select exists (select from project_automation where project_id = $1)`, p.ID).Scan(&on)
	if err != nil || !on || a.RunnerURL == "" {
		return err
	}
	var status string
	var current bool // the run tested the head its pull request has now
	err = tx.QueryRow(ctx, `
		select r.status, coalesce(r.head_sha = pr.head_sha, false) from test_runs r
		left join pull_requests pr on pr.project_id = $2 and pr.number = r.pr_number
		where r.task_id = $1 order by r.id desc limit 1`, taskID, p.ID).Scan(&status, &current)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) { // no rows: never tested
		return err
	}
	switch {
	case byHand && from == "testing" && (status == "queued" || status == "running"):
		return runInProgress
	case byHand && to == "user-check" && (status != "passed" || !current):
		return notTested
	case from == "testing":
		_, err := tx.Exec(ctx, `update tasks set needs_attention = null where id = $1`, taskID)
		return err
	case byHand && to == "testing":
		return startLinked(ctx, tx, p, taskID)
	}
	return nil
}

// startLinked starts a test run of task taskID for its linked pull request as the records have it: the one open,
// ready pull request closing its issue that may run, at its head. None, or more than one, starts a run that ends at
// once as a runner error. Call it in tx holding projects.Lock.
// shortcut: "may run" is the records' word, a branch of the repository whose refusal comment was never posted, as
// the triggers count ready pull requests; the author's write access is not asked again. Ask GitHub here if a
// pull request by an author without it can reach a task's record unrefused.
func startLinked(ctx context.Context, tx pgx.Tx, p projects.Project, taskID int64) error {
	type pull struct {
		Number int
		Head   string
	}
	rows, _ := tx.Query(ctx, `
		select r.number, r.head_sha from pull_requests r join tasks t on t.id = $2
		where r.project_id = $1 and not r.draft and t.issue_number = any(r.linked_issues) and r.head_repo_id = $3
			and not r.refused
		order by r.number limit 2`, p.ID, taskID, p.RepoID)
	pulls, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pull])
	if err != nil {
		return err
	}
	switch len(pulls) {
	case 0:
		return startRun(ctx, tx, p.ID, taskID, 0, "", "No linked pull request.")
	case 1:
		return startRun(ctx, tx, p.ID, taskID, pulls[0].Number, pulls[0].Head, "")
	}
	return startRun(ctx, tx, p.ID, taskID, 0, "", ambiguous)
}
