package automation_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"gantt-stop-me/api/internal/github/githubtest"
)

// takeover sends method, POST or DELETE, to issue 1's task's takeover route as session and returns the task it answers.
func (f *fixture) takeover(method string, session *http.Cookie) shownTask {
	f.t.Helper()
	res := f.Do(method, f.path(fmt.Sprintf("/tasks/%d/takeover", f.taskID(1))), nil, session)
	res.Expect(http.StatusOK, "")
	var got shownTask
	res.JSON(&got)
	return got
}

func TestTakeOver(t *testing.T) {
	f := on(t)
	n := f.pull("ada", false, 1)
	f.poll() // the task moves to Testing with a queued run
	f.exec(`update test_runs set status = 'running'`)
	bob, bobID, _ := f.member("bob", "push")

	got := f.takeover("POST", bob)
	if got.TakenOverBy == nil || got.TakenOverBy.ID != bobID || got.TakenOverBy.Login == nil || *got.TakenOverBy.Login != "bob" {
		t.Fatalf("takenOverBy = %+v, want bob", got.TakenOverBy)
	}
	if len(got.Assignees) != 1 || got.Assignees[0].Login != "bob" || !slices.Equal(f.repo.Issue(1).Assignees, []string{"bob"}) {
		t.Fatalf("assignees = %+v, on GitHub %v; want bob", got.Assignees, f.repo.Issue(1).Assignees)
	}
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "cancelled", Reason: "Taken over by bob.", Finished: true})

	// Taking it over again, by anyone, leaves it as it is.
	if again := f.takeover("POST", f.ada); again.TakenOverBy == nil || again.TakenOverBy.ID != bobID {
		t.Fatalf("taken over again: takenOverBy = %+v, want bob still", again.TakenOverBy)
	}
	if got := f.repo.Issue(1).Assignees; !slices.Equal(got, []string{"bob"}) {
		t.Fatalf("assignees on GitHub = %v, want bob alone", got)
	}

	// A pull request that becomes ready starts no run for it and leaves it where it is.
	f.move(1, "in-progress")
	f.pull("ada", false, 1)
	f.poll()
	if runs := f.runs(); len(runs) != 1 || f.task(1).Column != "in-progress" {
		t.Fatalf("after a new ready pull request: runs %+v, task %+v; want neither changed", runs, f.task(1))
	}
}

func TestTakenOverMovesSkipTheGuards(t *testing.T) {
	f := on(t)
	f.pull("ada", false, 1)
	f.poll()
	f.takeover("POST", f.ada)
	f.exec(`insert into test_runs (task_id, status) select id, 'running' from tasks`)
	f.move(1, "user-check") // out of Testing with a running run, into User Check untested
	f.move(1, "testing")    // starts no run
	if runs := f.runs(); len(runs) != 2 {
		t.Fatalf("runs = %+v, want no new one from the move into Testing", runs)
	}

	if got := f.takeover("DELETE", f.ada); got.TakenOverBy != nil {
		t.Fatalf("given back: takenOverBy = %+v, want null", got.TakenOverBy)
	}
	if runs := f.runs(); len(runs) != 2 {
		t.Fatalf("runs = %+v, want giving it back to start none", runs)
	}
	f.moveTo(1, "user-check", nil).Expect(http.StatusConflict, "run_in_progress")
	f.exec(`update test_runs set status = 'cancelled'`)
	f.moveTo(1, "user-check", nil).Expect(http.StatusConflict, "not_tested")
}

func TestTakeOverNeedsWriteAccess(t *testing.T) {
	f := on(t)
	reader, _, _ := f.member("bob", "pull")
	stranger, _, _ := f.member("eve", "")
	for _, method := range []string{"POST", "DELETE"} {
		path := f.path(fmt.Sprintf("/tasks/%d/takeover", f.taskID(1)))
		f.Do(method, path, nil, reader).Expect(http.StatusForbidden, "forbidden")
		f.Do(method, path, nil, stranger).Expect(http.StatusNotFound, "not_found")
		f.Do(method, path, nil).Expect(http.StatusUnauthorized, "unauthorized")
		f.Do(method, f.path("/tasks/999/takeover"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	}
	if len(f.repo.Issue(1).Assignees) != 0 {
		t.Fatal("a refused takeover assigned the issue")
	}
}

func TestTakenOverStartsNoRunFromAWebhook(t *testing.T) {
	f := automated(t)
	f.takeover("POST", f.ada)
	f.hint(f.pull("ada", false, 1))
	if runs := f.runs(); len(runs) != 0 || f.task(1).Column != "backlog" {
		t.Fatalf("after a ready pull request's webhook: runs %+v, task %+v; want neither changed", runs, f.task(1))
	}
}

// The takeover is done once GitHub has the assignee: a failed read of the pull requests after it answers it anyway.
func TestTakeOverWhenPullRequestsFail(t *testing.T) {
	f := on(t)
	f.GitHub.Fail("POST /graphql", http.StatusBadGateway)
	if got := f.takeover("POST", f.ada); got.TakenOverBy == nil {
		t.Fatal("takenOverBy is null, want ada")
	}
}

// A sync that fetched the issue before a takeover must not write the old assignees back over it.
func TestTakeOverSurvivesASyncThatFetchedBeforeIt(t *testing.T) {
	f := on(t)
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Title = "One!" }) // so the listing misses the ETag
	var once sync.Once
	var status int
	next := f.Config.GitHub.HTTP.Transport
	f.Config.GitHub.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		res, err := next.RoundTrip(r)
		if err == nil && r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/issues") {
			once.Do(func() {
				status = f.Do("POST", f.path(fmt.Sprintf("/tasks/%d/takeover", f.taskID(1))), nil, f.ada).Status
			})
		}
		return res, err
	})}
	f.exec(`update projects set synced_at = now() - interval '1 minute'`)
	f.Do("GET", f.path("/board"), nil, f.ada).Expect(http.StatusOK, "")
	if status != http.StatusOK {
		t.Fatalf("takeover during the sync answered %d", status)
	}
	var got shownTask
	f.Do("GET", f.path(fmt.Sprintf("/tasks/%d", f.taskID(1))), nil, f.ada).JSON(&got)
	if len(got.Assignees) != 1 || got.Assignees[0].Login != "ada" {
		t.Fatalf("assignees = %+v after the sync, want ada", got.Assignees)
	}
}
