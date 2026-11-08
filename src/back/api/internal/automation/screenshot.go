package automation

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// screenshot answers an LLM check's screenshot as PNG, also once automation is off.
func (a *Automation) screenshot(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	var png []byte
	err := a.DB.QueryRow(r.Context(), `
		select x.screenshot from test_results x join test_runs r on r.id = x.run_id join tasks t on t.id = r.task_id
		where x.id = $1 and x.run_id = $2 and t.project_id = $3 and x.screenshot is not null`,
		pathID(r, "resultID"), pathID(r, "runID"), p.ID).Scan(&png)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No screenshot.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Content-Type-Options", "nosniff") // the bytes are the browser's, so never read as anything else
	w.Write(png)
}
