package board

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// editTask changes the title or body of a task's issue on GitHub, then the cached title.
func (b *Board) editTask(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Title *string `json:"title"`
		Body  *string `json:"body"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	fields := map[string]string{}
	if in.Title != nil {
		title, ok := validTitle(w, *in.Title)
		if !ok {
			return
		}
		fields["title"] = title
	}
	if in.Body != nil {
		fields["body"] = *in.Body
	}
	if len(fields) == 0 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Change the title or the body.")
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
	is, err := b.GitHub.UpdateIssue(ctx, token, p.Owner, p.Name, t.IssueNumber, fields)
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	t, err = b.scanTask(b.DB.QueryRow(ctx, `update tasks t set title = $2 where t.id = $1 returning `+taskColumns, t.ID, is.Title), p)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}
