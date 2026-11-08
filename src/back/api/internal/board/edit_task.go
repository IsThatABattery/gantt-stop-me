package board

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// editTask changes the title, body or assignees of a task's issue on GitHub, then the cached title, body, labels and
// assignees, and takes the card's tests from the body. GitHub drops an assignee who cannot be assigned, so the answer
// shows who was.
func (b *Board) editTask(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Title     *string   `json:"title"`
		Body      *string   `json:"body"`
		Assignees *[]string `json:"assignees"` // logins; replaces the issue's assignees
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	fields := map[string]any{}
	if in.Title != nil {
		title, ok := validTitle(w, *in.Title)
		if !ok {
			return
		}
		fields["title"] = title
	}
	if in.Body != nil {
		if !validBody(w, *in.Body) {
			return
		}
		fields["body"] = *in.Body
	}
	if in.Assignees != nil {
		logins, ok := validAssignees(w, *in.Assignees)
		if !ok {
			return
		}
		fields["assignees"] = logins
	}
	if len(fields) == 0 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Change the title, the body or the assignees.")
		return
	}
	// Under the project's lock, as a tests edit is, so neither writes the issue body over the other's.
	unlock, err := projects.Serialize(r.Context(), p.ID)
	if err != nil {
		return // the client gave up waiting
	}
	defer unlock()
	// Once GitHub has the edit, finish saving it even if the browser goes away.
	ctx := context.WithoutCancel(r.Context())
	tx, err := b.DB.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer tx.Rollback(ctx)
	if err := projects.Lock(ctx, tx, p.ID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	t, err := b.scanTask(tx.QueryRow(ctx, taskByID, taskID(r), p.ID), p)
	if errors.Is(err, pgx.ErrNoRows) {
		taskNotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	is, err := b.GitHub.UpdateIssue(ctx, token, p.RepoID, t.IssueNumber, fields)
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	// updated_at, as a move sets it: a sync that fetched the issue before this edit then skips the task instead of undoing it.
	t, err = b.scanTask(tx.QueryRow(ctx, `
		update tasks t set title = $2, body = $3, labels = $4, assignees = $5, updated_at = clock_timestamp()
		where t.id = $1 returning `+taskColumns, t.ID, is.Title, is.Body, projects.Labels(is), projects.Assignees(is)), p)
	if err == nil {
		_, err = projects.TakeTests(ctx, tx, t.ID, is.Body)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

// maxAssignees is GitHub's limit on an issue's assignees.
const maxAssignees = 10

// login is a GitHub login, at most 39 characters: letters, digits and hyphens, and underscores for an Enterprise Managed
// User's handle_shortcode and older logins. GitHub itself drops a login it cannot assign.
var login = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,38}$`)

// validAssignees checks a task's new assignees, or answers 400 invalid_assignees and returns false.
func validAssignees(w http.ResponseWriter, logins []string) ([]string, bool) {
	if len(logins) > maxAssignees {
		httpx.Error(w, http.StatusBadRequest, "invalid_assignees", fmt.Sprintf("Assign at most %d people.", maxAssignees))
		return nil, false
	}
	for i, l := range logins {
		if !login.MatchString(l) || slices.Contains(logins[:i], l) {
			httpx.Error(w, http.StatusBadRequest, "invalid_assignees", "Each assignee must be a GitHub login, once.")
			return nil, false
		}
	}
	return logins, true
}
