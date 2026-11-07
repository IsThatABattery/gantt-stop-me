package board

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// show returns the project's columns and tasks, after a sync with GitHub if one is due.
func (b *Board) show(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Read)
	if !ok || !b.syncIfDue(w, r, p, token) {
		return
	}
	ctx := r.Context()
	cols, err := columns(ctx, b.DB, p.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	rows, _ := b.DB.Query(ctx, `
		select `+taskColumns+` from tasks t join columns c on c.id = t.column_id
		where t.project_id = $1 order by c.position, t.position, t.id`, p.ID)
	tasks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (task, error) { return b.scanTask(row, p) })
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"project": p, "columns": cols, "tasks": tasks})
}
