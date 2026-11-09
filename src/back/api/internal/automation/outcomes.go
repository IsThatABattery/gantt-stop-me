package automation

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/projects"
)

// over holds for test run r once it is planned and every workflow execution of it has ended.
const over = `exists (select from test_results x where x.run_id = r.id)
	and not exists (select from workflow_executions x where x.run_id = r.id and x.status in ('queued', 'running'))`

// finishAll finishes every test run of an automated, unpaused project that is over.
func (a *Automation) finishAll(ctx context.Context) {
	rows, _ := a.DB.Query(ctx, `
		select r.id from test_runs r join tasks t on t.id = r.task_id
		join project_automation pa on pa.project_id = t.project_id and pa.paused_reason is null
		where r.status in ('queued', 'running') and `+over+` order by r.id`)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	report(ctx, "list test runs to finish", err)
	for _, id := range ids {
		report(ctx, fmt.Sprintf("finish test run %d", id), a.finish(ctx, id, ""))
	}
}

// finish ends test run runID: with reason, a runner error for the whole run, or else once it is over, by its test
// results as Outcomes in docs/api/automation.md says. It applies the outcome to the task under the project's lock,
// as ordinary moves with the owner's credential, unless someone moved the task out of Testing meanwhile: a test
// failure moves it to In Progress, a runner error leaves it in Testing with needs_attention set to the first error's
// reason, and a pass moves it to User Check. A test failure also turns its pull request back into a draft, wherever
// the task is, since the pull request is still ready with failing tests. A run already finished, or not over, stays
// as it is. While the owner is paused, or GitHub fails the move for now, it returns an error and the next pass tries
// again.
func (a *Automation) finish(ctx context.Context, runID int64, reason string) error {
	var p projects.Project
	var taskID int64
	var pr int
	err := a.DB.QueryRow(ctx, `
		select p.id, p.github_repo_id, p.owner, p.name, t.id, coalesce(r.pr_number, 0)
		from test_runs r join tasks t on t.id = r.task_id join projects p on p.id = t.project_id where r.id = $1`, runID).
		Scan(&p.ID, &p.RepoID, &p.Owner, &p.Name, &taskID, &pr)
	if err != nil {
		return err
	}
	token, err := a.OwnerToken(ctx, p.ID)
	if err != nil {
		return err
	}
	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return err
	}
	ctx = context.WithoutCancel(ctx) // once GitHub has the move, finish saving it
	draft := false
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		var status, why, column string
		err := tx.QueryRow(ctx, `
			update test_runs r set finished_at = now(), reason = nullif($2, ''),
				status = case when $2 <> '' then 'error' else (select `+outcome+` from test_results where run_id = r.id) end
			where r.id = $1 and r.status in ('queued', 'running') and ($2 <> '' or `+over+`)
			returning r.status,
				coalesce(r.reason, (select detail from test_results where run_id = r.id and status not in ('passed', 'manual', 'failed')
					order by position limit 1), ''),
				(select c.key from tasks t join columns c on c.id = t.column_id where t.id = r.task_id)`, runID, reason).
			Scan(&status, &why, &column)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if draft = status == "failed" && pr != 0; column != "testing" {
			return nil
		}
		switch status {
		case "failed":
			err = a.Board.MoveToEnd(ctx, tx, token, p, taskID, "in-progress")
		case "error":
			_, err = tx.Exec(ctx, `update tasks set needs_attention = $2 where id = $1`, taskID, why)
		case "passed":
			err = a.Board.MoveToEnd(ctx, tx, token, p, taskID, "user-check")
		}
		if lasting(err) { // no retry would move it, so the run ends with the task where it is
			log.Printf("move the task of test run %d: %v", runID, err)
			return nil
		}
		return err
	})
	unlock() // the draft's GitHub calls need no lock
	if err != nil {
		return a.PauseIfRejected(ctx, p.ID, token, err)
	}
	if draft {
		a.draft(ctx, p, token, pr)
	}
	return nil
}

// draft turns pull request number back into a draft after a test failure, unless it is closed or a draft already,
// and saves that in its record, under the project's lock: so a run started before the triggers next read GitHub
// does not count it as ready, and they see no change to act on. A failure is logged, and GitHub's is returned for a
// caller that can tell someone.
func (a *Automation) draft(ctx context.Context, p projects.Project, token string, number int) error {
	pr, err := a.GitHub.PullRequest(ctx, token, p.RepoID, number)
	if err == nil && (pr.State != "open" || pr.Draft) {
		return nil
	}
	if err == nil {
		err = a.GitHub.ConvertToDraft(ctx, token, pr.NodeID)
	}
	if err != nil {
		err = a.PauseIfRejected(ctx, p.ID, token, err)
		log.Printf("turn pull request %d of project %d back into a draft: %v", number, p.ID, err)
		return err
	}
	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return nil // GitHub has it as a draft
	}
	defer unlock()
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `update pull_requests set draft = true where project_id = $1 and number = $2`, p.ID, number)
		return err
	})
	if err != nil {
		log.Printf("record pull request %d of project %d as a draft: %v", number, p.ID, err)
	}
	return nil
}

// commentAll posts the comment of every finished test run of an automated, unpaused project that has none yet,
// except a cancelled one, which gets none.
func (a *Automation) commentAll(ctx context.Context) {
	rows, _ := a.DB.Query(ctx, `
		select r.id from test_runs r join tasks t on t.id = r.task_id
		join project_automation pa on pa.project_id = t.project_id and pa.paused_reason is null
		where r.status in ('passed', 'failed', 'error') and r.commented_at is null order by r.id`)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	report(ctx, "list test runs to comment on", err)
	for _, id := range ids {
		report(ctx, fmt.Sprintf("comment on test run %d", id), a.comment(ctx, id))
	}
}

// comment posts finished test run runID's one comment on its issue, with the owner's credential: a line per test with
// its result, its text cut to one line, its workflow and links to its video and screenshot when it has them, the run's
// reason, the manual tests for the approver, and a link to the run. It marks the run commented right after, so a
// failed post is tried again on the next pass and a posted one never twice. GitHub refusing the post in a way no retry
// mends, such as for a locked issue, gives it up.
func (a *Automation) comment(ctx context.Context, runID int64) error {
	var p projects.Project
	var taskID int64
	var issue int
	var status, head, why string
	err := a.DB.QueryRow(ctx, `
		select p.id, p.github_repo_id, t.id, t.issue_number, r.status, coalesce(r.head_sha, ''), coalesce(r.reason, '')
		from test_runs r join tasks t on t.id = r.task_id join projects p on p.id = t.project_id where r.id = $1`, runID).
		Scan(&p.ID, &p.RepoID, &taskID, &issue, &status, &head, &why)
	if err != nil {
		return err
	}
	token, err := a.OwnerToken(ctx, p.ID)
	if err != nil {
		return err
	}
	title := map[string]string{
		"passed": "**Tests passed**", "failed": "**Tests failed**", "error": "**Tests could not finish: runner error**",
	}[status]
	if head != "" {
		title += " on " + head[:min(7, len(head))]
	}
	var lines, manual []string
	rows, _ := a.DB.Query(ctx, `
		select x.id, x.status, x.text, x.detail, coalesce(e.workflow, ''), x.video is not null, x.screenshot is not null
		from test_results x left join workflow_executions e on e.id = x.execution_id where x.run_id = $1 order by x.position`, runID)
	var id int64
	var result, text, detail, workflow string
	var video, screenshot bool
	_, err = pgx.ForEachRow(rows, []any{&id, &result, &text, &detail, &workflow, &video, &screenshot}, func() error {
		line := fmt.Sprintf("- %s: %s (%s)", result, oneLine(text), cmp.Or(workflow, "no workflow"))
		if result == "error" {
			first, _, _ := strings.Cut(detail, "\n")
			line += ": " + first
		}
		// Links, since GitHub's API cannot upload attachments. They need an account with read access, as the run's does.
		at := fmt.Sprintf("%s/api/projects/%d/runs/%d/results/%d/", a.PublicURL, p.ID, runID, id)
		if video {
			line += " [video](" + at + "video)"
		}
		if screenshot {
			line += " [screenshot](" + at + "screenshot)"
		}
		lines = append(lines, line)
		if result == "manual" {
			manual = append(manual, "- "+oneLine(text))
		}
		return nil
	})
	if err != nil {
		return err
	}
	parts := []string{title + "."}
	if len(lines) > 0 {
		parts = append(parts, strings.Join(lines, "\n"))
	}
	if why != "" {
		parts = append(parts, why)
	}
	if len(manual) > 0 {
		parts = append(parts, "Check these by hand before approving:\n"+strings.Join(manual, "\n"))
	}
	parts = append(parts, fmt.Sprintf("%s/projects/%d/tasks/%d?run=%d", a.PublicURL, p.ID, taskID, runID))

	if err := a.GitHub.CreateComment(ctx, token, p.RepoID, issue, strings.Join(parts, "\n\n")); err != nil {
		if err = a.PauseIfRejected(ctx, p.ID, token, err); !lasting(err) {
			return err
		}
		log.Printf("comment on test run %d: %v; giving up", runID, err)
	}
	_, err = a.DB.Exec(context.WithoutCancel(ctx), `update test_runs set commented_at = now() where id = $1`, runID)
	return err
}

// oneLine is the first line of s, cut to 100 characters.
func oneLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	if r := []rune(s); len(r) > 100 {
		s = string(r[:99]) + "…"
	}
	return s
}
