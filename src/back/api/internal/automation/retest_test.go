package automation_test

import (
	"net/http"
	"testing"
)

func TestRetest(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.work()
	f.expectRuns(pass(n))

	// A new run of the same head, leaving the card in User Check where the passed run put it.
	f.Do("POST", f.path("/tasks/1/retest"), nil, f.ada).Expect(http.StatusOK, "")
	f.expectRuns(pass(n), testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})
	f.work()
	f.expectRuns(pass(n), pass(n))

	bob, _, _ := f.member("bob", "pull")
	f.Do("POST", f.path("/tasks/1/retest"), nil, bob).Expect(http.StatusForbidden, "forbidden")
	f.Do("POST", f.path("/tasks/999/retest"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
}

func TestRetestNeedsAutomation(t *testing.T) {
	f := setup(t)
	f.Do("POST", f.path("/tasks/1/retest"), nil, f.ada).Expect(http.StatusConflict, "automation_off")
}

func TestRetestOutsideTestingNeedsNoAttention(t *testing.T) {
	f := on(t)
	f.Do("POST", f.path("/tasks/1/retest"), nil, f.ada).Expect(http.StatusOK, "")
	f.expectRuns(testRun{Issue: 1, Status: "error", Reason: "No linked pull request.", Finished: true})
	// No move out of Testing would ever clear it.
	if s := f.task(1); s.NeedsAttention != "" {
		t.Fatalf("a retest of a task in %s flagged it %q", s.Column, s.NeedsAttention)
	}
}
