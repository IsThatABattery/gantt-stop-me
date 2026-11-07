package board_test

import (
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/github/githubtest"
)

func TestGetTask(t *testing.T) {
	f := setup(t, "One")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "The details."; is.Labels = []string{"bug", "board:backlog"} })
	id, _ := f.task(1)

	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.reader())
	res.Expect(http.StatusOK, "")
	var got struct {
		task
		Body   string
		Labels []struct{ Name, Color string }
	}
	res.JSON(&got)
	if got.ID != id || got.IssueNumber != 1 || got.Title != "One" || got.ColumnID != f.column("backlog") || got.URL == "" {
		t.Fatalf("task = %+v", got.task)
	}
	if got.Body != "The details." || len(got.Labels) != 2 || got.Labels[0].Name != "bug" || got.Labels[0].Color != "ededed" ||
		got.Labels[1].Name != "board:backlog" || got.Labels[1].Color != "c5def5" {
		t.Fatalf("body %q, labels %+v", got.Body, got.Labels)
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

func TestGetTaskWhenGitHubFails(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.GitHub.Fail("GET /repos/{owner}/{name}/issues/{number}", http.StatusInternalServerError)
	f.Do("GET", f.path("/tasks/%d", id), nil, f.ada).Expect(http.StatusBadGateway, "github_error")
}
