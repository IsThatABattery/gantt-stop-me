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
// test run for the task's linked pull request, whose author it checks on GitHub with token. Any move out of Testing
// clears needs_attention. A task a member took over moves as the API's own moves do: no guard and no test run.
// It implements board.Rules.
func (a *Automation) Moving(ctx context.Context, tx pgx.Tx, token string, p projects.Project, taskID int64, from, to string, byHand bool) error {
	var on, takenOver bool
	err := tx.QueryRow(ctx, `select exists (select from project_automation where project_id = $1),
		exists (select from tasks where id = $2 and taken_over_by is not null)`, p.ID, taskID).Scan(&on, &takenOver)
	if err != nil || !on || a.RunnerURL == "" {
		return err
	}
	byHand = byHand && !takenOver
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
		return a.startLinked(ctx, tx, token, p, taskID)
	}
	return nil
}

// startLinked starts a test run of task taskID for its linked pull request: the one open, ready pull request closing
// its issue that may run, at its head as the records have it. None, or more than one, starts a run that ends at once
// as a runner error. A record says a pull request comes from a branch of the repository, but not that its author can
// push: one first seen ready while it closed no open task's issue was never asked about. So it asks GitHub with
// token, as the triggers do. Call it in tx holding projects.Lock.
// shortcut: the triggers' count of ready pull requests still takes the records' word, so such a pull request can
// turn a trusted one's run into the ambiguous error; ask GitHub there too if that is abused.
func (a *Automation) startLinked(ctx context.Context, tx pgx.Tx, token string, p projects.Project, taskID int64) error {
	type pull struct {
		Number int
		Head   string
	}
	rows, _ := tx.Query(ctx, `
		select r.number, r.head_sha from pull_requests r join tasks t on t.id = $2
		where r.project_id = $1 and not r.draft and t.issue_number = any(r.linked_issues) and r.head_repo_id = $3
			and not r.refused
		order by r.number`, p.ID, taskID, p.RepoID)
	pulls, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pull])
	if err != nil {
		return err
	}
	var trusted []pull // two at most: enough to tell one from several
	for _, r := range pulls {
		pr, err := a.GitHub.PullRequest(ctx, token, p.RepoID, r.Number)
		if err != nil {
			return err
		}
		if pr.State != "open" || pr.Draft { // since the triggers last read it
			continue
		}
		why, err := a.refusal(ctx, token, p, pr)
		if err != nil {
			return err
		}
		if why == "" {
			if trusted = append(trusted, r); len(trusted) == 2 {
				break
			}
		}
	}
	switch len(trusted) {
	case 0:
		return startRun(ctx, tx, p.ID, taskID, 0, "", "No linked pull request.")
	case 1:
		return startRun(ctx, tx, p.ID, taskID, trusted[0].Number, trusted[0].Head, "")
	}
	return startRun(ctx, tx, p.ID, taskID, 0, "", ambiguous)
}
