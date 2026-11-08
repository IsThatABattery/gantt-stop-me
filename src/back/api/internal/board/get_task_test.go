package board_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/github/githubtest"
)

type taskDetails struct {
	task
	Body   string
	Labels []label
}

func TestGetTask(t *testing.T) {
	f := setup(t, "One")
	f.exec(`update tasks set body = 'The details.', labels = '[{"name": "bug", "color": "d73a4a"}, {"name": "board:backlog", "color": "c5def5"}]'`)
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "Only on GitHub" }) // no sync is due, so the row answers
	id, _ := f.task(1)

	n := len(f.GitHub.Requests())
	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.reader())
	res.Expect(http.StatusOK, "")
	var got taskDetails
	res.JSON(&got)
	if got.ID != id || got.IssueNumber != 1 || got.Title != "One" || got.State != "open" || got.ColumnID != f.column("backlog") || got.URL == "" {
		t.Fatalf("task = %+v", got.task)
	}
	if want := []label{{"bug", "d73a4a"}, {"board:backlog", "c5def5"}}; got.Body != "The details." || !slices.Equal(got.Labels, want) {
		t.Fatalf("body %q, labels %+v; want the stored ones", got.Body, got.Labels)
	}
	for _, r := range f.since(n) {
		if strings.HasPrefix(r, "GET /repositories/1/issues/") {
			t.Fatalf("GET task fetched the issue from GitHub: %v", f.since(n))
		}
	}
}

func TestGetTaskRunStatus(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	get := func() task {
		res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
		res.Expect(http.StatusOK, "")
		var got task
		res.JSON(&got)
		return got
	}
	if got := get(); got.RunStatus != nil || got.NeedsAttention != nil {
		t.Fatalf("untested task = %+v, want null run status and needs attention", got)
	}
	f.runs(1, "", "error", "passed")
	if got := get(); orNull(got.RunStatus) != "passed" || got.NeedsAttention != nil {
		t.Fatalf("task = %+v, want the latest run's status", got)
	}
}

func TestGetTaskSyncsWhenDue(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.repo.EditIssue(1, func(is *githubtest.Issue) {
		is.Title, is.State, is.Body, is.Labels = "Renamed", "closed", "New body", []string{"bug"}
	})
	get := func() taskDetails {
		t.Helper()
		res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
		res.Expect(http.StatusOK, "")
		var got taskDetails
		res.JSON(&got)
		return got
	}

	if got := get(); got.Title != "One" || got.State != "open" || got.Body != "" || len(got.Labels) != 0 {
		t.Fatalf("within 30 seconds of the last sync: %+v, want the cached task", got)
	}
	f.dueSync()
	if got := get(); got.Title != "Renamed" || got.State != "closed" || got.ColumnID != f.column("closed") || got.Body != "New body" ||
		!slices.Equal(got.Labels, []label{{"bug", "ededed"}}) {
		t.Fatalf("after a due sync: %+v, want the issue as GitHub has it", got)
	}
}

func TestGetTaskHidesLabelIDs(t *testing.T) {
	f := setup(t, "One")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Labels = []string{"bug"} })
	f.dueSync()
	f.board()
	id, _ := f.task(1)
	var stored bool
	f.DB.QueryRow(t.Context(), `select labels->0->'id' is not null from tasks where id = $1`, id).Scan(&stored)
	if !stored {
		t.Fatal("the sync stored the label without its id")
	}

	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var got struct{ Labels []map[string]any }
	res.JSON(&got)
	if len(got.Labels) != 1 || len(got.Labels[0]) != 2 || got.Labels[0]["name"] != "bug" || got.Labels[0]["color"] != "ededed" {
		t.Fatalf("labels = %v, want only the name and color of bug", got.Labels)
	}
}

func TestGetTaskUnknown(t *testing.T) {
	f := setup(t, "One")
	f.Do("GET", f.path("/tasks/999"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path("/tasks/nope"), nil, f.ada).Expect(http.StatusNotFound, "not_found")

	// A task of another project is not found through this one.
	other := f.GitHub.AddRepo("octo", "other")
	other.Grant(f.token, "push")
	other.AddIssue("Elsewhere")
	otherID := f.AddProject(f.ada, "octo", "other")
	var foreign int64
	f.DB.QueryRow(t.Context(), `select id from tasks where project_id = $1`, otherID).Scan(&foreign)
	f.Do("GET", f.path("/tasks/%d", foreign), nil, f.ada).Expect(http.StatusNotFound, "not_found")
}

func TestGetTaskNeedsAccess(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.Do("GET", f.path("/tasks/%d", id), nil, f.stranger()).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path("/tasks/%d", id), nil).Expect(http.StatusUnauthorized, "unauthorized")
}

func TestGetTaskWithRevokedCredential(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.dueSync()
	f.GitHub.Fail("GET /repositories/{id}/issues", http.StatusUnauthorized) // the token expired or was revoked
	f.Do("GET", f.path("/tasks/%d", id), nil, f.ada).Expect(http.StatusForbidden, "github_not_linked")
}

func TestGetTaskWhenSyncFails(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "Not fetched" })
	f.dueSync()
	f.GitHub.Fail("GET /repositories/{id}/issues", http.StatusInternalServerError)
	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var got taskDetails
	res.JSON(&got)
	if got.ID != id || got.Body != "" {
		t.Fatalf("task = %+v, want the cached one", got)
	}
}
