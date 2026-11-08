package automation

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/projects"
)

// takeOver marks a task taken over by the caller, a write member, so automation leaves it to them: its queued and
// running test runs are cancelled as turning automation off cancels them, pull requests start none for it, and its
// moves skip the guards. The issue gets the caller as an assignee. A task taken over already stays as it is. It
// answers the task as Get task does.
func (a *Automation) takeOver(w http.ResponseWriter, r *http.Request) {
	p, token, ok := a.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	ctx := r.Context()
	taskID := pathID(r, "taskID")
	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return // the client gave up waiting
	}
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		var issue int
		var taken bool
		err := tx.QueryRow(ctx, `select issue_number, taken_over_by is not null from tasks where id = $1 and project_id = $2`,
			taskID, p.ID).Scan(&issue, &taken)
		if errors.Is(err, pgx.ErrNoRows) {
			return noTask
		}
		if err != nil || taken {
			return err
		}
		var login string
		if err := tx.QueryRow(ctx, `select login from github_links where user_id = $1`, accounts.UserID(ctx)).Scan(&login); err != nil {
			return err
		}
		is, err := a.GitHub.AddAssignees(ctx, token, p.RepoID, issue, []string{login})
		if err != nil {
			return gitHubError{err}
		}
		// updated_at, as a move sets it: a sync that fetched the issue before this then skips the task instead of undoing it.
		_, err = tx.Exec(ctx, `update tasks set taken_over_by = $2, assignees = $3, updated_at = clock_timestamp() where id = $1`,
			taskID, accounts.UserID(ctx), projects.Assignees(is))
		if err != nil {
			return err
		}
		return cancelRuns(ctx, tx, p.ID, taskID, 0, "Taken over by "+login+".")
	})
	unlock()
	a.answerTask(w, r, p, token, taskID, err)
}

// giveBack hands a task taken over back to automation. It starts no test run by itself. It answers the task as Get
// task does.
func (a *Automation) giveBack(w http.ResponseWriter, r *http.Request) {
	p, token, ok := a.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	taskID := pathID(r, "taskID")
	tag, err := a.DB.Exec(r.Context(), `update tasks set taken_over_by = null where id = $1 and project_id = $2`, taskID, p.ID)
	if err == nil && tag.RowsAffected() == 0 {
		err = noTask
	}
	a.answerTask(w, r, p, token, taskID, err)
}
