// Package projects adds GitHub repositories as projects and checks who may open them.
package projects

import (
	"context"
	"net/http"
	"strings"
	"sync"

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

// Lock serializes, until tx ends, the writes that place a project's tasks or write their issues: moves, syncs,
// imports, new tasks and task and tests edits, so a version check, a GitHub write and a save cannot interleave with
// another. Writers queue in Serialize first,
// so a waiting one does not hold a pooled connection.
func Lock(ctx context.Context, tx pgx.Tx, projectID int64) error {
	_, err := tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, projectID)
	return err
}

// gates holds one gate per project, so a slow write to one project never delays another.
// shortcut: a gate is never freed, one small channel per project written since start; prune if projects number in the millions.
var gates sync.Map // project id -> chan struct{}

// Serialize waits, before the writer takes a DB connection, until no other write to the project runs in this
// process, or until ctx ends. The writer calls unlock when done, and still takes Lock in its transaction.
// shortcut: in-process only; the advisory lock still serializes across api instances, where waiters would pin
// connections again. Upgrade to a queue outside the pool before running more than one instance.
func Serialize(ctx context.Context, projectID int64) (unlock func(), err error) {
	v, _ := gates.LoadOrStore(projectID, make(chan struct{}, 1))
	g := v.(chan struct{})
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

// Assignee is an issue's assignee as the API shows and caches it.
type Assignee struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatarUrl"`
}

// Assignees returns the issue's assignees, empty rather than nil, which would store NULL.
func Assignees(is github.Issue) []Assignee {
	out := []Assignee{}
	for _, u := range is.Assignees {
		out = append(out, Assignee{u.Login, u.AvatarURL})
	}
	return out
}

// Labels returns the issue's labels, empty rather than nil, which would store NULL.
func Labels(is github.Issue) []github.Label {
	if is.Labels == nil {
		return []github.Label{}
	}
	return is.Labels
}
