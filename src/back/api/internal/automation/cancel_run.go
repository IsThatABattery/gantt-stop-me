package automation

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// cancelRun cancels a queued or running test run, with its workflow executions, which the queue worker then stops,
// and skips its unfinished tests. Its reason names the caller. The task stays where it is, and the run lock goes
// with the run. It holds the project's lock, as the triggers and finishing a run do.
func (a *Automation) cancelRun(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	ctx := r.Context()
	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return // the client gave up waiting
	}
	defer unlock()
	var status string
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		var taskID int64
		err := tx.QueryRow(ctx, `select r.task_id, r.status from test_runs r join tasks t on t.id = r.task_id
			where r.id = $1 and t.project_id = $2`, pathID(r, "runID"), p.ID).Scan(&taskID, &status)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && status != "queued" && status != "running" {
			return nil
		}
		if err != nil {
			return err
		}
		var login string
		if err := tx.QueryRow(ctx, `select login from github_links where user_id = $1`, accounts.UserID(ctx)).Scan(&login); err != nil {
			return err
		}
		// A task has one queued or running run at most, since starting one cancels the others: this one.
		return cancelRuns(ctx, tx, p.ID, taskID, 0, "Cancelled by "+login+".")
	})
	switch {
	case err != nil:
		httpx.Internal(w, r, err)
	case status == "":
		runNotFound(w)
	case status != "queued" && status != "running":
		httpx.Error(w, http.StatusConflict, "run_finished", "This test run has already ended.")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
