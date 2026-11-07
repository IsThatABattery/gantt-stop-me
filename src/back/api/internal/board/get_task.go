package board

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// getTask returns a task with its issue's body and labels, which only GitHub has.
func (b *Board) getTask(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	ctx := r.Context()
	t, err := b.scanTask(b.DB.QueryRow(ctx, taskByID, taskID(r), p.ID), p)
	if errors.Is(err, pgx.ErrNoRows) {
		taskNotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	is, err := b.GitHub.Issue(ctx, token, p.Owner, p.Name, t.IssueNumber)
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, struct {
		task
		Body   string         `json:"body"`
		Labels []github.Label `json:"labels"`
	}{t, is.Body, is.Labels})
}
