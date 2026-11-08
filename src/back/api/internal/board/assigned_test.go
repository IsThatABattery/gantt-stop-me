package board_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

type assignedList struct {
	Projects []struct {
		Project struct{ Name string }
		Tasks   []struct {
			IssueNumber int
			URL         string
			RunStatus   *string
			Column      struct{ Key, Name string }
		}
	}
}

func TestAssignedTasks(t *testing.T) {
	f := setup(t, "Mine", "Theirs", "Mine too", "Done")
	world := f.GitHub.AddRepo("octo", "world")
	world.Grant(f.token, "push")
	world.AddIssue("Elsewhere")
	worldID := f.AddProject(f.ada, "octo", "world")
	bob := f.reader()
	assign := func(path string, logins ...string) {
		f.t.Helper()
		f.Do("PATCH", path, map[string]any{"assignees": logins}, f.ada).Expect(http.StatusOK, "")
	}
	var worldTask int64
	if err := f.DB.QueryRow(context.Background(), `select id from tasks where project_id = $1`, worldID).Scan(&worldTask); err != nil {
		t.Fatal(err)
	}
	assign(fmt.Sprintf("/api/projects/%d/tasks/%d", worldID, worldTask), "ada-gh")
	for _, n := range []int{1, 3, 4} {
		id, _ := f.task(n)
		assign(f.path("/tasks/%d", id), "ada-gh")
	}
	id2, _ := f.task(2)
	assign(f.path("/tasks/%d", id2), "bob-gh")

	id3, _ := f.task(3)
	f.exec(`update tasks set column_id = $1 where id = $2`, f.column("testing"), id3)
	f.exec(`insert into test_runs (task_id, status) values ($1, 'running')`, id3)
	id4, _ := f.task(4)
	f.exec(`update tasks set state = 'closed' where id = $1`, id4)

	// Opening a board is a visit: hello was opened last, so it comes first.
	f.Do("GET", fmt.Sprintf("/api/projects/%d/board", worldID), nil, f.ada).Expect(http.StatusOK, "")
	f.Do("GET", f.path("/board"), nil, f.ada).Expect(http.StatusOK, "")
	res := f.Do("GET", "/api/me/tasks", nil, f.ada)
	res.Expect(http.StatusOK, "")
	var got assignedList
	res.JSON(&got)
	if len(got.Projects) != 2 || got.Projects[0].Project.Name != "hello" || got.Projects[1].Project.Name != "world" {
		t.Fatalf("projects = %+v", got.Projects)
	}
	hello := got.Projects[0].Tasks
	if len(hello) != 2 || hello[0].IssueNumber != 1 || hello[0].Column.Key != "backlog" || hello[0].RunStatus != nil ||
		hello[1].IssueNumber != 3 || hello[1].Column.Name != "Testing" || orNull(hello[1].RunStatus) != "running" ||
		hello[1].URL != f.GitHub.Server.URL+"/octo/hello/issues/3" {
		t.Fatalf("hello's tasks = %+v", hello)
	}

	// Bob has not opened the project, so his task there is not listed until he does.
	var bobs assignedList
	f.Do("GET", "/api/me/tasks", nil, bob).JSON(&bobs)
	if len(bobs.Projects) != 0 {
		t.Fatalf("bob's before opening = %+v", bobs.Projects)
	}
	f.Do("GET", f.path("/board"), nil, bob).Expect(http.StatusOK, "")
	f.Do("GET", "/api/me/tasks", nil, bob).JSON(&bobs)
	if len(bobs.Projects) != 1 || len(bobs.Projects[0].Tasks) != 1 || bobs.Projects[0].Tasks[0].IssueNumber != 2 {
		t.Fatalf("bob's after opening = %+v", bobs.Projects)
	}

	// Losing access to world drops its tasks once the cached permission expires.
	world.Grant(f.token, "")
	f.exec(`update project_members set checked_at = now() - interval '5 minutes 1 second'`)
	got = assignedList{}
	f.Do("GET", "/api/me/tasks", nil, f.ada).JSON(&got)
	if len(got.Projects) != 1 || got.Projects[0].Project.Name != "hello" {
		t.Fatalf("after losing world = %+v", got.Projects)
	}

	f.Do("GET", "/api/me/tasks", nil).Expect(http.StatusUnauthorized, "unauthorized")
	f.Do("GET", "/api/me/tasks", nil, f.SignUp("eve@example.com", "correct horse")).Expect(http.StatusForbidden, "github_not_linked")
}
