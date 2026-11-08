package automation_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"gantt-stop-me/api/internal/github/githubtest"
)

// taskID is the id of issue number's task.
func (f *fixture) taskID(number int) int64 {
	f.t.Helper()
	var id int64
	if err := f.DB.QueryRow(context.Background(), `select id from tasks where project_id = $1 and issue_number = $2`, f.project, number).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	return id
}

// lastRun is the id of the newest test run.
func (f *fixture) lastRun() int64 {
	f.t.Helper()
	var id int64
	if err := f.DB.QueryRow(context.Background(), `select max(id) from test_runs`).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	return id
}

// listedRun is a test run as the run endpoints answer it.
type listedRun struct {
	ID         int64
	PRNumber   *int
	HeadSHA    *string
	Status     string
	Reason     *string
	CreatedAt  time.Time
	FinishedAt *time.Time
}

func TestListRuns(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = true })
	f.poll() // cancels the first run
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft, p.HeadSHA = false, "b2" })
	f.poll() // starts the second
	f.work()
	f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "") // runs stay readable once it is off

	reader, _, _ := f.member("bob", "pull")
	res := f.Do("GET", f.path(fmt.Sprintf("/tasks/%d/runs", f.taskID(1))), nil, reader)
	res.Expect(http.StatusOK, "")
	var got struct{ Runs []listedRun }
	res.JSON(&got)
	if len(got.Runs) != 2 {
		t.Fatalf("runs = %+v, want two", got.Runs)
	}
	second, first := got.Runs[0], got.Runs[1] // newest first
	if second.Status != "passed" || *second.PRNumber != n || *second.HeadSHA != "b2" || second.Reason != nil || second.FinishedAt == nil {
		t.Fatalf("newest run = %+v", second)
	}
	if first.Status != "cancelled" || *first.HeadSHA != "a1" || *first.Reason != "The pull request went back to draft." ||
		first.ID >= second.ID || first.CreatedAt.IsZero() {
		t.Fatalf("oldest run = %+v", first)
	}

	res = f.Do("POST", f.path("/tasks"), map[string]string{"title": "Two"}, f.ada)
	res.Expect(http.StatusCreated, "")
	var untested struct{ ID int64 }
	res.JSON(&untested)
	res = f.Do("GET", f.path(fmt.Sprintf("/tasks/%d/runs", untested.ID)), nil, reader)
	res.Expect(http.StatusOK, "")
	if strings.TrimSpace(string(res.Body)) != `{"runs":[]}` {
		t.Fatalf("an untested task's runs = %s", res.Body)
	}
}

func TestListRunsNeedsAccess(t *testing.T) {
	f := withLLM(t)
	f.Do("GET", f.path("/tasks/999/runs"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	stranger := f.SignUp("eve@example.com", "correct horse")
	f.LinkGitHub(stranger, "eve")
	f.Do("GET", f.path(fmt.Sprintf("/tasks/%d/runs", f.taskID(1))), nil, stranger).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path(fmt.Sprintf("/tasks/%d/runs", f.taskID(1))), nil).Expect(http.StatusUnauthorized, "unauthorized")
}
