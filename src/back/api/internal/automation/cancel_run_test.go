package automation_test

import (
	"fmt"
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

// cancel asks to cancel the newest test run as session.
func (f *fixture) cancel(session *http.Cookie) *apptest.Response {
	f.t.Helper()
	return f.Do("POST", f.path(fmt.Sprintf("/runs/%d/cancel", f.lastRun())), nil, session)
}

func TestCancelRun(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.plan() // a queued execution
	reader, _, _ := f.member("bob", "pull")
	f.cancel(reader).Expect(http.StatusForbidden, "forbidden")
	f.cancel(f.ada).Expect(http.StatusNoContent, "")

	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "cancelled", Reason: "Cancelled by ada.", Finished: true})
	f.expectExecutions(executionRow{"default", "cancelled", 0})
	f.expectResults(resultRow{1, "skipped", "", "default", false}, resultRow{2, "skipped", "", "default", false})
	if got := f.task(1); got.Column != "testing" {
		t.Fatalf("task = %+v, want it left in Testing", got)
	}
	f.cancel(f.ada).Expect(http.StatusConflict, "run_finished")
	f.work()
	f.expectCalls()
	f.expectComments()
	f.move(1, "in-progress") // unlocked
}

func TestCancelRunWhileRunning(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	f.ready()
	h := f.Runner.Hold(0, "command")
	wait := f.workInBackground()
	<-h.Arrived()
	f.cancel(f.ada).Expect(http.StatusNoContent, "")
	wait() // the worker noticed and stopped
	f.expectCalls("prepare 1", "command 1 go test ./...", "delete 1")
	f.expectExecutions(executionRow{"default", "cancelled", 1})
	f.expectResults(resultRow{1, "skipped", "", "default", false}, resultRow{2, "skipped", "", "default", false})
}

func TestCancelRunNotFound(t *testing.T) {
	f := withLLM(t)
	f.Do("POST", f.path("/runs/999/cancel"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
}
