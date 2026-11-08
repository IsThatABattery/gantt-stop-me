package automation

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// listRuns answers a task's test runs, newest first, also once automation is off.
func (a *Automation) listRuns(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	ctx := r.Context()
	taskID := pathID(r, "taskID")
	var found bool
	if err := a.DB.QueryRow(ctx, `select exists (select from tasks where id = $1 and project_id = $2)`, taskID, p.ID).Scan(&found); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if !found {
		httpx.Error(w, http.StatusNotFound, "not_found", "Task not found.")
		return
	}
	rows, _ := a.DB.Query(ctx, `select `+runColumns+` from test_runs r where r.task_id = $1 order by r.id desc`, taskID)
	runs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[run])
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"runs": runs})
}
