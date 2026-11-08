package automation_test

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/runner/runnertest"
)

// movedTask is the task the move endpoint answers, with what automation adds.
type movedTask struct {
	RunStatus, NeedsAttention *string
}

func TestRunLock(t *testing.T) {
	f := withLLM(t)
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Two"}, f.ada).Expect(http.StatusCreated, "")
	f.tests(1, twoTests)
	n := f.ready() // in Testing with a queued run
	before := f.task(1)
	for _, key := range []string{"in-progress", "user-check", "closed"} {
		f.moveTo(1, key, nil).Expect(http.StatusConflict, "run_in_progress")
	}
	if got := f.task(1); got != before {
		t.Fatalf("task = %+v, want it left as %+v", got, before)
	}
	if labels := f.repo.Issue(1).Labels; !slices.Equal(labels, []string{"board:testing"}) {
		t.Fatalf("issue labels = %v, want GitHub untouched", labels)
	}

	f.move(2, "testing")
	two := 2
	f.moveTo(1, "testing", &two).Expect(http.StatusOK, "") // a reorder within Testing is no move out

	var runID int64
	f.DB.QueryRow(t.Context(), `select max(id) from test_runs where pr_number = $1`, n).Scan(&runID)
	f.Do("POST", f.path(fmt.Sprintf("/runs/%d/cancel", runID)), nil, f.ada).Expect(http.StatusNoContent, "")
	f.move(1, "in-progress") // unlocked
}

func TestRunLockWhileRunning(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	f.ready()
	h := f.Runner.Hold(0, "command")
	wait := f.workInBackground()
	<-h.Arrived()
	f.moveTo(1, "backlog", nil).Expect(http.StatusConflict, "run_in_progress")
	h.Release()
	wait()
	f.move(1, "in-progress") // the run passed and moved it to User Check, from where it moves freely
}

func TestUserCheckGate(t *testing.T) {
	f := withLLM(t)
	f.moveTo(1, "user-check", nil).Expect(http.StatusConflict, "not_tested") // never tested

	f.tests(1, twoTests)
	n := f.ready()
	f.work() // passes, and the API moves it to User Check itself
	if got := f.task(1); got.Column != "user-check" {
		t.Fatalf("task = %+v, want the pass to move it to User Check", got)
	}
	f.move(1, "in-progress")
	f.move(1, "user-check") // passed on the pull request's current head

	f.move(1, "in-progress")
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.HeadSHA = "b2" })
	f.poll()
	f.moveTo(1, "user-check", nil).Expect(http.StatusConflict, "not_tested") // the head moved since

	f.Runner.Script(0, runnertest.Script{Commands: map[string]runnertest.Exit{"go test ./...": {Code: 1}}})
	f.move(1, "testing")
	f.work()
	f.expectRuns(pass(n), testRun{Issue: 1, PR: n, Head: "b2", Status: "failed", Finished: true})
	f.moveTo(1, "user-check", nil).Expect(http.StatusConflict, "not_tested") // failed on the current head
}

func TestPassMovesToUserCheckPastTheGate(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.plan()
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.HeadSHA = "b2" }) // pushed to while the run tests a1
	f.poll()
	f.work()
	f.expectRuns(pass(n))
	if got := f.task(1); got.Column != "user-check" {
		t.Fatalf("task = %+v, want the API's own move on a pass to skip the gate", got)
	}
}

func TestMoveIntoTestingStartsARun(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.work()
	f.move(1, "in-progress")

	res := f.moveTo(1, "testing", nil)
	res.Expect(http.StatusOK, "")
	var got movedTask
	res.JSON(&got)
	if got.RunStatus == nil || *got.RunStatus != "queued" || got.NeedsAttention != nil {
		t.Fatalf("moved task = %+v, want a queued run", got)
	}
	f.expectRuns(pass(n), testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})
	f.work()
	f.expectRuns(pass(n), pass(n))
}

func TestMoveIntoTestingWithoutOneLinkedPullRequest(t *testing.T) {
	for _, c := range []struct {
		name   string
		pulls  func(f *fixture)
		reason string
	}{
		{"none", func(f *fixture) {}, "No linked pull request."},
		{"a draft", func(f *fixture) { f.pull("ada", true, 1) }, "No linked pull request."},
		{"from a fork", func(f *fixture) {
			f.repo.AddPull("Fork", githubtest.Pull{Author: "ada", HeadSHA: "f1", HeadRepo: 999, Closes: f.closing(1)})
		}, "No linked pull request."},
		{"two", func(f *fixture) { f.pull("ada", false, 1); f.pull("ada", false, 1) }, ambiguous},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := withLLM(t)
			f.place("closed") // so the pull requests' triggers start no run
			c.pulls(f)
			f.poll()
			f.place("backlog")

			res := f.moveTo(1, "testing", nil)
			res.Expect(http.StatusOK, "")
			var got movedTask
			res.JSON(&got)
			if got.RunStatus == nil || *got.RunStatus != "error" || got.NeedsAttention == nil || *got.NeedsAttention != c.reason {
				t.Fatalf("moved task = %+v, want a runner error %q", got, c.reason)
			}
			f.expectRuns(testRun{Issue: 1, Status: "error", Reason: c.reason, Finished: true})
			f.work()
			f.expectComments("**Tests could not finish: runner error**.\n\n" + c.reason + "\n\n" + f.link())

			f.move(1, "in-progress") // a runner error does not lock the task
			if got := f.task(1); got.NeedsAttention != "" {
				t.Fatalf("task = %+v, want leaving Testing to clear needs_attention", got)
			}
		})
	}
}

func TestMovesWithoutAutomation(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(t *testing.T) *fixture
	}{
		{"never on", func(t *testing.T) *fixture { return setup(t) }},
		{"turned off", func(t *testing.T) *fixture {
			f := withLLM(t)
			f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "")
			return f
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := c.setup(t)
			f.unlinkedRun() // in Testing with a queued run, as automation left it
			f.exec(`update tasks set needs_attention = 'An old runner error.'`)
			f.move(1, "user-check")
			f.move(1, "testing")
			f.move(1, "in-progress")
			if runs := f.runs(); len(runs) != 1 {
				t.Fatalf("runs = %+v, want no new one", runs)
			}
			if got := f.task(1); got.NeedsAttention != "An old runner error." {
				t.Fatalf("task = %+v, want automation's flag left alone", got)
			}
		})
	}
}
