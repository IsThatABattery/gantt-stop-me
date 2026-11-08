package board

import (
	"context"
	"errors"
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
	id, err := b.CreateTask(ctx, token, p, title, in.Body)
	var t task
	if err == nil {
		t, err = b.scanTask(b.DB.QueryRow(ctx, taskByID, id, p.ID), p)
	}
	switch {
	case errors.As(err, new(GitHubError)):
		github.Fail(w, r, err)
	case ctx.Err() != nil: // the client gave up waiting; the next sync imports the issue
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		httpx.JSON(w, http.StatusCreated, t)
	}
}

// CreateTask opens an issue with title and body, labeled board:backlog, in the project's repository with token, then
// adds its task at the end of Backlog, its card taking the body's tests, and returns the task's id. A failed GitHub
// call comes back as a GitHubError. The caller checks title and body fit GitHub.
func (b *Board) CreateTask(ctx context.Context, token string, p projects.Project, title, body string) (int64, error) {
	is, err := b.GitHub.CreateIssue(ctx, token, p.RepoID, title, body, []string{"board:backlog"})
	if err != nil {
		return 0, GitHubError{err}
	}
	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return 0, err
	}
	defer unlock()
	var id int64
	err = pgx.BeginFunc(ctx, b.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		// A sync may have imported the new issue already; its task is then the one returned.
		if err := projects.AddTask(ctx, tx, p.ID, is); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `select id from tasks where github_issue_id = $1 and project_id = $2`, is.ID, p.ID).Scan(&id)
	})
	return id, err
}
