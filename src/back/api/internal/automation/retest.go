package automation

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/projects"
)

var (
	automationOff   = &refused{http.StatusConflict, "automation_off", "Turn automation on first."}
	retestTakenOver = &refused{http.StatusConflict, "taken_over",
		"A member took this task over from automation, so it runs no tests. Give it back to retest it."}
)

// retest answers POST /api/projects/{id}/tasks/{taskID}/retest: a new test run of the task's linked pull request at
// its head, as a move into Testing starts, cancelling the task's earlier queued or running one. The card stays where
// it is, so a run's outcome moves it only from Testing.
func (a *Automation) retest(w http.ResponseWriter, r *http.Request) {
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
		var on, taken bool
		err := tx.QueryRow(ctx, `select exists (select from project_automation where project_id = $2), taken_over_by is not null
			from tasks where id = $1 and project_id = $2`, taskID, p.ID).Scan(&on, &taken)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return noTask
		case err != nil:
			return err
		case !on:
			return automationOff
		case taken:
			return retestTakenOver
		}
		return a.startLinked(ctx, tx, token, p, taskID)
	})
	unlock()
	a.answerTask(w, r, p, token, taskID, err)
}
