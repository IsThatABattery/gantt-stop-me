package board_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestCreateTask(t *testing.T) {
	f := setup(t, "One")
	res := f.Do("POST", f.path("/tasks"), map[string]string{"title": "  Write docs \n", "body": "All of them."}, f.ada)
	res.Expect(http.StatusCreated, "")
	var got task
	res.JSON(&got)
	if got.ID == 0 || got.IssueNumber != 2 || got.Title != "Write docs" || got.State != "open" || got.Version != 1 ||
		got.ColumnID != f.column("backlog") || got.URL != f.GitHub.Server.URL+"/octo/hello/issues/2" {
		t.Fatalf("task = %+v", got)
	}
	is := f.repo.Issue(2)
	if is.Title != "Write docs" || is.Body != "All of them." || is.State != "open" || !slices.Equal(is.Labels, []string{"board:backlog"}) {
		t.Fatalf("issue on GitHub = %+v", is)
	}
	if order := f.order("backlog"); !slices.Equal(order, []int{1, 2}) {
		t.Fatalf("backlog = %v, want the new task last", order)
	}
	if body, labels := f.details(2); body != "All of them." || !slices.Equal(labels, []label{{"board:backlog", "c5def5"}}) {
		t.Fatalf("stored body %q, labels %v; want the created issue's", body, labels)
	}
}

func TestCreateTaskValidatesTitle(t *testing.T) {
	f := setup(t)
	for _, title := range []string{"", "   ", strings.Repeat("é", 257)} {
		f.Do("POST", f.path("/tasks"), map[string]string{"title": title}, f.ada).Expect(http.StatusBadRequest, "invalid_title")
	}
	f.Do("POST", f.path("/tasks"), "{", f.ada).Expect(http.StatusBadRequest, "bad_request")
	f.Do("POST", f.path("/tasks"), map[string]string{"title": strings.Repeat("é", 256)}, f.ada).Expect(http.StatusCreated, "")
	if n := len(f.board().Tasks); n != 1 {
		t.Fatalf("%d tasks, want only the valid one", n)
	}
}

func TestCreateTaskValidatesBody(t *testing.T) {
	f := setup(t)
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Long", "body": strings.Repeat("é", 65537)}, f.ada).
		Expect(http.StatusBadRequest, "invalid_body")
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Fits", "body": strings.Repeat("é", 65536)}, f.ada).
		Expect(http.StatusCreated, "")
	if n := len(f.board().Tasks); n != 1 {
		t.Fatalf("%d tasks, want only the one that fits", n)
	}
}

func TestCreateTaskNeedsWrite(t *testing.T) {
	f := setup(t)
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Nope"}, f.reader()).Expect(http.StatusForbidden, "forbidden")
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Nope"}, f.stranger()).Expect(http.StatusNotFound, "not_found")
	if n := len(f.board().Tasks); n != 0 {
		t.Fatalf("%d tasks created", n)
	}
}

func TestCreateTaskWhenGitHubFails(t *testing.T) {
	f := setup(t)
	f.GitHub.Fail("POST /repositories/{id}/issues", http.StatusInternalServerError)
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Lost"}, f.ada).Expect(http.StatusBadGateway, "github_error")
	f.GitHub.Fail("POST /repositories/{id}/issues", http.StatusForbidden)
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Lost"}, f.ada).Expect(http.StatusForbidden, "forbidden")
	if n := len(f.board().Tasks); n != 0 {
		t.Fatalf("%d tasks stored", n)
	}
}

func TestCreateTaskNeedsSession(t *testing.T) {
	f := setup(t)
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Nope"}).Expect(http.StatusUnauthorized, "unauthorized")
}
