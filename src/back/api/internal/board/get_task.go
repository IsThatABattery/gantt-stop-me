package board

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// getTask returns a task with its issue's cached body and labels, after a sync with GitHub if one is due.
func (b *Board) getTask(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Read)
	if !ok || !b.syncIfDue(w, r, p, token) {
		return
	}
	var out struct {
		task
		Body   string         `json:"body"`
		Labels []github.Label `json:"labels"`
	}
	var err error
	out.task, err = b.scanTask(b.DB.QueryRow(r.Context(), `
		select `+taskColumns+`, t.body, t.labels from tasks t where t.id = $1 and t.project_id = $2`, taskID(r), p.ID),
		p, &out.Body, &out.Labels)
	if errors.Is(err, pgx.ErrNoRows) {
		taskNotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
