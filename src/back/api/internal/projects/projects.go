// Package projects adds GitHub repositories as projects and checks who may open them.
package projects

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/secret"
)

// Projects serves POST /api/projects and the access check every project route starts with.
type Projects struct {
	DB       *pgxpool.Pool
	Accounts *accounts.Accounts // guards every route
	Secrets  *secret.Box        // opens stored GitHub credentials
	GitHub   *github.Client
}

// Project is a repository added to the app, with the signed-in user's permission on it.
type Project struct {
	ID         int64  `json:"id"`
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	Permission string `json:"permission"` // "read", "write" or "admin"
}

// Register adds the projects routes to mux.
func (p *Projects) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/projects", p.Accounts.Require(p.add))
}

// Lock serializes, until tx ends, the writes that place a project's tasks: moves, syncs, imports and new tasks,
// so a version check, a GitHub write and a save cannot interleave with another.
func Lock(ctx context.Context, tx pgx.Tx, projectID int64) error {
	_, err := tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, projectID)
	return err
}
