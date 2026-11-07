package board

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// createTask opens an issue labeled board:backlog and adds its task at the end of Backlog.
func (b *Board) createTask(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	title, ok := validTitle(w, in.Title)
	if !ok || !validBody(w, in.Body) {
		return
	}
	ctx := r.Context()
	is, err := b.GitHub.CreateIssue(ctx, token, p.RepoID, title, in.Body, []string{"board:backlog"})
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return // the client gave up waiting; the next sync imports the issue
	}
	defer unlock()
	var t task
	err = pgx.BeginFunc(ctx, b.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		// A sync may have imported the new issue already; its task is then the one returned.
		if err := projects.AddTask(ctx, tx, p.ID, is); err != nil {
			return err
		}
		var err error
		t, err = b.scanTask(tx.QueryRow(ctx, `select `+taskColumns+` from tasks t where t.github_issue_id = $1 and t.project_id = $2`, is.ID, p.ID), p)
		return err
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}
