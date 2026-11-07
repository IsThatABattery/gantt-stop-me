package board_test

import (
	"net/http"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/github/githubtest"
)

func TestEditTask(t *testing.T) {
	f := setup(t, "One")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "Old body." })
	id, _ := f.task(1)

	res := f.Do("PATCH", f.path("/tasks/%d", id), map[string]string{"title": " Renamed "}, f.ada)
	res.Expect(http.StatusOK, "")
	var got task
	res.JSON(&got)
	if got.ID != id || got.Title != "Renamed" || got.Version != 1 {
		t.Fatalf("task = %+v", got)
	}
	if is := f.repo.Issue(1); is.Title != "Renamed" || is.Body != "Old body." {
		t.Fatalf("issue = %+v, want the title changed and the body kept", is)
	}
	if _, s := f.task(1); s.Title != "Renamed" {
		t.Fatalf("cached title = %q", s.Title)
	}

	f.Do("PATCH", f.path("/tasks/%d", id), map[string]string{"body": ""}, f.ada).Expect(http.StatusOK, "")
	if is := f.repo.Issue(1); is.Title != "Renamed" || is.Body != "" {
		t.Fatalf("issue = %+v, want the body cleared and the title kept", is)
	}
}

func TestEditTaskValidates(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.Do("PATCH", f.path("/tasks/%d", id), map[string]any{}, f.ada).Expect(http.StatusBadRequest, "bad_request")
	f.Do("PATCH", f.path("/tasks/%d", id), map[string]any{"title": " ", "body": "ok"}, f.ada).Expect(http.StatusBadRequest, "invalid_title")
	f.Do("PATCH", f.path("/tasks/%d", id), map[string]any{"body": strings.Repeat("é", 65537)}, f.ada).Expect(http.StatusBadRequest, "invalid_body")
	f.Do("PATCH", f.path("/tasks/%d", id), "{", f.ada).Expect(http.StatusBadRequest, "bad_request")
	f.Do("PATCH", f.path("/tasks/999"), map[string]string{"title": "Lost"}, f.ada).Expect(http.StatusNotFound, "not_found")
	if is := f.repo.Issue(1); is.Title != "One" || is.Body != "" {
		t.Fatalf("issue = %+v, want it unchanged", is)
	}
}

func TestEditTaskNeedsWrite(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.Do("PATCH", f.path("/tasks/%d", id), map[string]string{"title": "Nope"}, f.reader()).Expect(http.StatusForbidden, "forbidden")
	f.Do("PATCH", f.path("/tasks/%d", id), map[string]string{"title": "Nope"}).Expect(http.StatusUnauthorized, "unauthorized")
	if is := f.repo.Issue(1); is.Title != "One" {
		t.Fatalf("issue = %+v, want it unchanged", is)
	}
}

func TestEditTaskWhenGitHubFails(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.GitHub.Fail("PATCH /repositories/{id}/issues/{number}", http.StatusInternalServerError)
	f.Do("PATCH", f.path("/tasks/%d", id), map[string]string{"title": "Lost"}, f.ada).Expect(http.StatusBadGateway, "github_error")
	if _, s := f.task(1); s.Title != "One" {
		t.Fatalf("cached title = %q after GitHub failed, want One", s.Title)
	}
}
