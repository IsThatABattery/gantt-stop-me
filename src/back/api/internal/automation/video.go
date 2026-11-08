package automation

import (
	"bytes"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// video answers an LLM check's recording of the browser as WebM, also once automation is off. It answers Range
// requests, so a player can seek in it.
// shortcut: every request, each seek among them, reads the whole recording, up to 64 MiB, from Postgres; read only
// the range asked with substring() if seeking loads the database.
func (a *Automation) video(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	var webm []byte
	err := a.DB.QueryRow(r.Context(), `
		select x.video from test_results x join test_runs r on r.id = x.run_id join tasks t on t.id = r.task_id
		where x.id = $1 and x.run_id = $2 and t.project_id = $3 and x.video is not null`,
		pathID(r, "resultID"), pathID(r, "runID"), p.ID).Scan(&webm)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No video.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "video/webm")
	w.Header().Set("X-Content-Type-Options", "nosniff") // the bytes are the browser's, so never read as anything else
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(webm))
}
