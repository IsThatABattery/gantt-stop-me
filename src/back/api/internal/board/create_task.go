package board

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
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
	unlock, err := projects.Serialize(r.Context(), p.ID)
	if err != nil {
		return // the client gave up waiting
	}
	defer unlock()
	// Once GitHub has the issue, finish saving its task even if the browser goes away.
	ctx := context.WithoutCancel(r.Context())
	var t task
	err = pgx.BeginFunc(ctx, b.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		id, err := b.CreateTask(ctx, tx, token, p, title, in.Body)
		if err == nil {
			t, err = b.scanTask(tx.QueryRow(ctx, taskByID, id, p.ID), p)
		}
		return err
	})
	switch {
	case errors.As(err, new(GitHubError)):
		b.Projects.Credentials.Fail(w, r, accounts.UserID(r.Context()), token, err)
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		httpx.JSON(w, http.StatusCreated, t)
	}
}

// CreateTask opens an issue with title and body, labeled board:backlog, in the project's repository with token, then
// adds its task at the end of Backlog in tx, its card taking the body's tests, and returns the task's id. A failed
// GitHub call comes back as a GitHubError. Call it in tx holding projects.Lock, and commit tx only when it succeeds,
// so a caller can record the task in the same transaction. The caller checks title and body fit GitHub.
func (b *Board) CreateTask(ctx context.Context, tx pgx.Tx, token string, p projects.Project, title, body string) (int64, error) {
	is, err := b.GitHub.CreateIssue(ctx, token, p.RepoID, title, body, []string{"board:backlog"})
	if err != nil {
		return 0, GitHubError{err}
	}
	if err := projects.AddTask(ctx, tx, p.ID, is); err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRow(ctx, `select id from tasks where github_issue_id = $1 and project_id = $2`, is.ID, p.ID).Scan(&id)
	return id, err
}
