// Package projects adds GitHub repositories as projects and checks who may open them.
package projects

import (
	"context"
	"net/http"
	"strings"

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
	RepoID     int64  `json:"-"` // GitHub's id of the repository, which every GitHub call goes by
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	Permission string `json:"permission"` // "read", "write" or "admin"
}

// Register adds the projects routes to mux.
func (p *Projects) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/projects", p.Accounts.Require(p.add))
}

// Lock serializes, until tx ends, the writes that place a project's tasks: moves, syncs, imports and new tasks,
// so a version check, a GitHub write and a save cannot interleave with another. Writers queue in Serialize first,
// so a waiting one does not hold a pooled connection.
func Lock(ctx context.Context, tx pgx.Tx, projectID int64) error {
	_, err := tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, projectID)
	return err
}

// gates queue each project's writers in Serialize. Projects share them by id, which keeps their number fixed.
var gates = func() (g [64]chan struct{}) {
	for i := range g {
		g[i] = make(chan struct{}, 1)
	}
	return g
}()

// Serialize waits, before the writer takes a DB connection, until no other write to the project runs in this
// process, or until ctx ends. The writer calls unlock when done, and still takes Lock in its transaction.
// shortcut: in-process only; the advisory lock still serializes across api instances, where waiters would pin
// connections again. Upgrade to a queue outside the pool before running more than one instance.
func Serialize(ctx context.Context, projectID int64) (unlock func(), err error) {
	g := gates[uint64(projectID)%uint64(len(gates))]
	select {
	case g <- struct{}{}:
		return func() { <-g }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// BoardLabels returns the issue's board:* labels, lower-cased since GitHub label names ignore case.
func BoardLabels(is github.Issue) []string {
	labels := []string{} // not nil, which would store NULL
	for _, l := range is.Labels {
		if name := strings.ToLower(l.Name); strings.HasPrefix(name, "board:") {
			labels = append(labels, name)
		}
	}
	return labels
}
