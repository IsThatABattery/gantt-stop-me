package automation_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
)

// manualCheck is an issue body with a script and an LLM check that inUserCheck's fake LLM leaves to the approver.
const manualCheck = "## Tests\n- [ ] `go test ./...`\n- [ ] The sign-up page looks right\n"

// tickExactly is how every unchecked_manual_tests message starts.
const tickExactly = "Check each manual test of the latest test run by hand and tick exactly those before approving."

// inUserCheck is a fixture whose issue 1 task is in User Check after a test run of a ready pull request by ada, at
// head a1, passed with one script and one manual test. It returns the pull request's number.
func inUserCheck(t *testing.T) (*fixture, int) {
	f := withLLM(t)
	f.tests(1, manualCheck)
	f.llm.Reply(reported("unverifiable", "It needs a person's eye."))
	n := f.ready()
	f.work()
	if got := f.task(1); got.Column != "user-check" {
		t.Fatalf("task = %+v, want the pass to move it to User Check", got)
	}
	return f, n
}

// manual returns the ids of the newest test run's manual tests, as GET run lists them for the approver to tick.
func (f *fixture) manual() []int64 {
	f.t.Helper()
	res := f.Do("GET", f.path(fmt.Sprintf("/runs/%d", f.lastRun())), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var got shownRun
	res.JSON(&got)
	var ids []int64
	for _, x := range got.Results {
		if x.Status == "manual" {
			ids = append(ids, x.ID)
		}
	}
	return ids
}

// approve asks the approve endpoint, as session, to merge issue 1's task with checked ticked.
func (f *fixture) approve(session *http.Cookie, checked ...int64) *apptest.Response {
	f.t.Helper()
	return f.Do("POST", f.path(fmt.Sprintf("/tasks/%d/approve", f.taskID(1))), map[string]any{"checked": checked}, session)
}

// merges counts the merge requests the fake GitHub has served.
func (f *fixture) merges() int {
	n := 0
	for _, r := range f.GitHub.Requests() {
		if strings.HasPrefix(r, "PUT ") && strings.HasSuffix(r, "/merge") {
			n++
		}
	}
	return n
}

// approvals counts the stored test runs with an approval.
func (f *fixture) approvals() int {
	f.t.Helper()
	var n int
	if err := f.DB.QueryRow(context.Background(), `select count(*) from test_runs where approved_at is not null or approved_by is not null`).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// errorMessage is the error message of a refused request.
func errorMessage(res *apptest.Response) string {
	var body struct{ Error struct{ Message string } }
	res.JSON(&body)
	return body.Error.Message
}

// expectHeadMoved fails the test unless an approval of pull request n found its head moved to b2: nothing merged or
// approved, the task is back in Testing, and a test run of b2 is queued.
func (f *fixture) expectHeadMoved(n int) {
	f.t.Helper()
	if pr := f.repo.Issue(n); pr.State != "open" || pr.Pull.MergedWith != "" {
		f.t.Fatalf("pull request = %s, merged with %q; want it open", pr.State, pr.Pull.MergedWith)
	}
	if got := f.task(1); got.Column != "testing" {
		f.t.Fatalf("task = %+v, want it back in Testing", got)
	}
	if labels := f.repo.Issue(1).Labels; !slices.Equal(labels, []string{"board:testing"}) {
		f.t.Fatalf("issue labels = %v, want the move on GitHub", labels)
	}
	f.expectRuns(pass(n), testRun{Issue: 1, PR: n, Head: "b2", Status: "queued"})
	if got := f.approvals(); got != 0 {
		f.t.Fatalf("approved runs = %d, want none", got)
	}
}

func TestApprove(t *testing.T) {
	for _, c := range []struct {
		allowed []string
		want    string // the merge method
	}{
		{[]string{"squash", "merge", "rebase"}, "squash"},
		{[]string{"merge", "rebase"}, "merge"},
		{[]string{"rebase"}, "rebase"},
	} {
		t.Run(c.want, func(t *testing.T) {
			f, n := inUserCheck(t)
			f.repo.AllowMerges(c.allowed...)
			bob, _, _ := f.member("bob", "push") // any write member approves, not only the owner
			res := f.approve(bob, f.manual()...)
			res.Expect(http.StatusOK, "")
			var got listedRun
			res.JSON(&got)
			if got.ID != f.lastRun() || got.Status != "passed" || *got.HeadSHA != "a1" || got.ApprovedBy == nil ||
				*got.ApprovedBy != "bob" || got.ApprovedAt == nil {
				t.Fatalf("run = %+v, want it approved by bob", got)
			}
			if pr := f.repo.Issue(n); pr.State != "closed" || pr.Pull.MergedWith != c.want {
				t.Fatalf("pull request = %s, merged with %q; want it merged with %s", pr.State, pr.Pull.MergedWith, c.want)
			}
			if got := f.task(1); got.Column != "closed" {
				t.Fatalf("task = %+v, want it in Closed", got)
			}
			if is := f.repo.Issue(1); is.State != "closed" || !slices.Equal(is.Labels, []string{"board:closed"}) {
				t.Fatalf("issue = %s with %v, want the move on GitHub", is.State, is.Labels)
			}
			f.expectRuns(pass(n))

			res = f.Do("GET", f.path(fmt.Sprintf("/tasks/%d/runs", f.taskID(1))), nil, f.ada)
			var listed struct{ Runs []listedRun }
			res.JSON(&listed)
			if r := listed.Runs[0]; r.ApprovedBy == nil || *r.ApprovedBy != "bob" || !r.ApprovedAt.Equal(*got.ApprovedAt) {
				t.Fatalf("listed run = %+v, want its approval", r)
			}
		})
	}
}

func TestApproveHeadMoved(t *testing.T) {
	f, n := inUserCheck(t)
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.HeadSHA = "b2" })
	f.approve(f.ada, f.manual()...).Expect(http.StatusConflict, "head_moved")
	if got := f.merges(); got != 0 {
		t.Fatalf("merge requests = %d, want none", got)
	}
	f.expectHeadMoved(n)
}

func TestApproveHeadMovedAtMergeTime(t *testing.T) {
	f, n := inUserCheck(t)
	manual := f.manual()
	// Pushed to after the approval read the pull request, while it reads the repository's merge methods.
	next := f.Config.GitHub.HTTP.Transport
	f.Config.GitHub.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/graphql" {
			f.repo.EditPull(n, func(p *githubtest.Pull) { p.HeadSHA = "b2" })
		}
		return next.RoundTrip(r)
	})}
	f.approve(f.ada, manual...).Expect(http.StatusConflict, "head_moved")
	if got := f.merges(); got != 1 {
		t.Fatalf("merge requests = %d, want the one GitHub refused", got)
	}
	f.expectHeadMoved(n)
}

func TestApproveHeadMovedRetestsOnlyWhatMayRun(t *testing.T) {
	t.Run("draft", func(t *testing.T) {
		f, n := inUserCheck(t)
		f.repo.EditPull(n, func(p *githubtest.Pull) { p.HeadSHA, p.Draft = "b2", true })
		f.poll() // back in draft, with nothing to cancel
		f.approve(f.ada, f.manual()...).Expect(http.StatusConflict, "head_moved")
		const why = "The pull request is a draft. Mark it ready for review to test it."
		f.expectRuns(pass(n), testRun{Issue: 1, PR: n, Head: "b2", Status: "error", Reason: why, Finished: true})
		if got := f.task(1); got.Column != "testing" || got.NeedsAttention != why {
			t.Fatalf("task = %+v, want it in Testing, flagged", got)
		}
		f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = false })
		f.poll() // ready again: the trigger tests it
		f.expectRuns(pass(n), testRun{Issue: 1, PR: n, Head: "b2", Status: "error", Reason: why, Finished: true},
			testRun{Issue: 1, PR: n, Head: "b2", Status: "queued"})
	})
	t.Run("author lost write access", func(t *testing.T) {
		f := withLLM(t)
		f.tests(1, manualCheck)
		f.llm.Reply(reported("unverifiable", "It needs a person's eye."))
		_, _, bob := f.member("bob", "push")
		n := f.pull("bob", false, 1)
		f.poll()
		f.work()
		f.repo.Grant(bob, "pull")
		f.repo.EditPull(n, func(p *githubtest.Pull) { p.HeadSHA = "b2" })
		f.approve(f.ada, f.manual()...).Expect(http.StatusConflict, "head_moved")
		const why = "Not tested: bob does not have write access to this repository. Automation tests only pull requests by authors who can push to it."
		f.expectRuns(pass(n), testRun{Issue: 1, PR: n, Head: "b2", Status: "error", Reason: why, Finished: true})
		if got := f.task(1); got.Column != "testing" || got.NeedsAttention != why {
			t.Fatalf("task = %+v, want it in Testing, flagged", got)
		}
	})
}

func TestApproveMergeRefusedOnTheTestedHead(t *testing.T) {
	f, n := inUserCheck(t)
	manual := f.manual()
	// GitHub answers 409, but reading the pull request again still shows the tested head, as when the read lags a push.
	next := f.Config.GitHub.HTTP.Transport
	f.Config.GitHub.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge") {
			return &http.Response{StatusCode: http.StatusConflict, Header: http.Header{}, Request: r,
				Body: io.NopCloser(strings.NewReader(`{"message": "Head branch was modified. Review and try the merge again."}`))}, nil
		}
		return next.RoundTrip(r)
	})}
	f.approve(f.ada, manual...).Expect(http.StatusBadGateway, "github_error")
	if got := f.task(1); got.Column != "user-check" || f.approvals() != 0 {
		t.Fatalf("task = %+v with %d approvals, want it left in User Check unapproved", got, f.approvals())
	}
	f.expectRuns(pass(n)) // no run of the commit that just passed
}

func TestApproveKeepsAMergeWhoseAnswerIsLost(t *testing.T) {
	f, n := inUserCheck(t)
	manual := f.manual()
	next := f.Config.GitHub.HTTP.Transport
	f.Config.GitHub.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		res, err := next.RoundTrip(r)
		if err == nil && r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge") {
			res.Body.Close()
			return nil, errors.New("connection reset by peer")
		}
		return res, err
	})}
	f.approve(f.ada, manual...).Expect(http.StatusOK, "")
	if pr := f.repo.Issue(n); pr.State != "closed" || f.approvals() != 1 || f.task(1).Column != "closed" {
		t.Fatalf("pull request = %s with %d approvals, task in %s; want it merged, approved and closed", pr.State, f.approvals(), f.task(1).Column)
	}
}

func TestApproveRefusesWhatBranchProtectionBlocks(t *testing.T) {
	f, n := inUserCheck(t)
	f.repo.Grant(f.token, "admin") // the owner may bypass branch protection, so GitHub's merge would go through
	for _, state := range []string{"blocked", "behind", "dirty", "draft", "unknown"} {
		f.repo.EditPull(n, func(p *githubtest.Pull) { p.MergeState = state })
		res := f.approve(f.ada, f.manual()...)
		res.Expect(http.StatusConflict, "not_mergeable")
		if state == "blocked" {
			if got, want := errorMessage(res), "Branch protection blocks merging this pull request, such as for a required review or status check."; got != want {
				t.Fatalf("message = %q, want %q", got, want)
			}
		}
	}
	if got := f.merges(); got != 0 || f.task(1).Column != "user-check" {
		t.Fatalf("merge requests = %d, task in %s; want none, left in User Check", got, f.task(1).Column)
	}
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.MergeState = "unstable" }) // only checks that are not required failed
	f.approve(f.ada, f.manual()...).Expect(http.StatusOK, "")
}

func TestApproveTurnedOffMeanwhile(t *testing.T) {
	f, n := inUserCheck(t)
	manual := f.manual()
	// Turned off after the approval checked the owner, while it reads the repository's merge methods.
	next, off := f.Config.GitHub.HTTP.Transport, false
	f.Config.GitHub.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/graphql" && !off {
			off = true
			f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "")
		}
		return next.RoundTrip(r)
	})}
	f.approve(f.ada, manual...).Expect(http.StatusConflict, "automation_off")
	if pr := f.repo.Issue(n); pr.State != "open" || f.merges() != 0 || f.approvals() != 0 {
		t.Fatalf("pull request = %s with %d merge requests and %d approvals, want nothing merged", pr.State, f.merges(), f.approvals())
	}
}

func TestApproveNeedsEveryManualTick(t *testing.T) {
	f, n := inUserCheck(t)
	manual := f.manual()
	if len(manual) != 1 {
		t.Fatalf("manual tests = %v, want one", manual)
	}
	var script int64 // the passed script's result
	if err := f.DB.QueryRow(context.Background(), `select id from test_results where status = 'passed'`).Scan(&script); err != nil {
		t.Fatal(err)
	}
	const missing = ` Not ticked: "The sign-up page looks right".`
	extra := fmt.Sprintf(" Not its manual tests: %d.", script)
	for _, c := range []struct {
		name    string
		checked []int64
		message string
	}{
		{"none", nil, tickExactly + missing},
		{"a passed test too", []int64{manual[0], script}, tickExactly + extra},
		{"an unknown id too", []int64{manual[0], 999}, tickExactly + " Not its manual tests: 999."},
		{"a passed test instead", []int64{script}, tickExactly + missing + extra},
	} {
		res := f.approve(f.ada, c.checked...)
		res.Expect(http.StatusUnprocessableEntity, "unchecked_manual_tests")
		if got := errorMessage(res); got != c.message {
			t.Errorf("%s: message = %q, want %q", c.name, got, c.message)
		}
	}
	if got := f.merges(); got != 0 {
		t.Fatalf("merge requests = %d, want none", got)
	}
	f.approve(f.ada, manual[0], manual[0]).Expect(http.StatusOK, "") // a set: one tick twice is that tick
	if pr := f.repo.Issue(n); pr.Pull.MergedWith != "squash" {
		t.Fatalf("pull request merged with %q, want squash", pr.Pull.MergedWith)
	}
}

func TestApproveWithNoManualTests(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.work()
	f.approve(f.ada).Expect(http.StatusOK, "")
	if pr := f.repo.Issue(n); pr.State != "closed" {
		t.Fatalf("pull request = %s, want it merged", pr.State)
	}
}

func TestApproveNeedsAPassedRunInUserCheck(t *testing.T) {
	f := withLLM(t)
	f.approve(f.ada).Expect(http.StatusConflict, "not_tested") // never tested
	f.place("user-check")
	f.approve(f.ada).Expect(http.StatusConflict, "not_tested") // in User Check, as the issue sync can put it, untested

	f.tests(1, manualCheck)
	f.llm.Reply(reported("unverifiable", "It needs a person's eye."))
	f.ready()
	f.work()
	manual := f.manual()
	f.move(1, "in-progress")
	f.approve(f.ada, manual...).Expect(http.StatusConflict, "not_tested") // passed, but out of User Check

	f.move(1, "testing") // a new test run, queued
	f.place("user-check")
	f.approve(f.ada, manual...).Expect(http.StatusConflict, "not_tested") // its latest run has not passed
	if got := f.merges(); got != 0 {
		t.Fatalf("merge requests = %d, want none", got)
	}
}

func TestApproveNotMergeable(t *testing.T) {
	f, n := inUserCheck(t)
	const why = "Merge conflict between base and head"
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.Unmergeable = why })
	res := f.approve(f.ada, f.manual()...)
	res.Expect(http.StatusConflict, "not_mergeable")
	if got := errorMessage(res); got != why {
		t.Fatalf("message = %q, want GitHub's %q", got, why)
	}
	if got := f.task(1); got.Column != "user-check" {
		t.Fatalf("task = %+v, want it left in User Check", got)
	}
	if got := f.approvals(); got != 0 {
		t.Fatalf("approved runs = %d, want none", got)
	}

	f.repo.EditPull(n, func(p *githubtest.Pull) { p.Unmergeable = "" }) // the conflict resolved without a new head
	f.approve(f.ada, f.manual()...).Expect(http.StatusOK, "")
}

func TestApproveClosedPullRequest(t *testing.T) {
	f, n := inUserCheck(t)
	f.repo.EditIssue(n, func(is *githubtest.Issue) { is.State = "closed" })
	f.approve(f.ada, f.manual()...).Expect(http.StatusConflict, "pull_request_closed")
	if got := f.task(1); got.Column != "user-check" {
		t.Fatalf("task = %+v, want it left in User Check", got)
	}
}

func TestApproveWhilePausedOrOff(t *testing.T) {
	t.Run("credential rejected", func(t *testing.T) {
		f, _ := inUserCheck(t)
		manual := f.manual()
		f.GitHub.Fail("GET /repositories/{id}/pulls/{number}", http.StatusUnauthorized)
		f.approve(f.ada, manual...).Expect(http.StatusConflict, "automation_paused")
		if got := f.pausedReason(); got == nil || *got != "GitHub no longer accepts ada's credential." {
			t.Fatalf("paused_reason = %v, want the rejected credential", got)
		}
		f.GitHub.Fail("GET /repositories/{id}/pulls/{number}", 0)
		f.approve(f.ada, manual...).Expect(http.StatusConflict, "automation_paused") // until a takeover
		if got := f.merges(); got != 0 {
			t.Fatalf("merge requests = %d, want none", got)
		}
	})
	t.Run("off", func(t *testing.T) {
		f, _ := inUserCheck(t)
		manual := f.manual()
		f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "")
		f.approve(f.ada, manual...).Expect(http.StatusConflict, "automation_off")
	})
	t.Run("GitHub failing", func(t *testing.T) {
		f, _ := inUserCheck(t)
		manual := f.manual()
		f.GitHub.Fail("PUT /repositories/{id}/pulls/{number}/merge", http.StatusBadGateway)
		f.approve(f.ada, manual...).Expect(http.StatusBadGateway, "github_error")
		if got := f.task(1); got.Column != "user-check" || f.approvals() != 0 {
			t.Fatalf("task = %+v with %d approvals, want it left in User Check unapproved", got, f.approvals())
		}
	})
}

func TestApproveKeepsTheMergeWhenTheMoveFails(t *testing.T) {
	f, n := inUserCheck(t)
	f.GitHub.Fail("POST /repositories/{id}/issues/{number}/labels", http.StatusBadGateway)
	f.approve(f.ada, f.manual()...).Expect(http.StatusOK, "")
	if pr := f.repo.Issue(n); pr.State != "closed" || f.approvals() != 1 {
		t.Fatalf("pull request = %s with %d approvals, want it merged and the approval saved", pr.State, f.approvals())
	}
	if got := f.task(1); got.Column != "user-check" {
		t.Fatalf("task = %+v, want it left for the issue sync", got)
	}
}

func TestApproveNeedsWriteAccess(t *testing.T) {
	f, _ := inUserCheck(t)
	manual := f.manual()
	reader, _, _ := f.member("bob", "pull")
	f.approve(reader, manual...).Expect(http.StatusForbidden, "forbidden")
	stranger := f.SignUp("eve@example.com", "correct horse")
	f.LinkGitHub(stranger, "eve")
	f.approve(stranger, manual...).Expect(http.StatusNotFound, "not_found")
	f.approve(nil, manual...).Expect(http.StatusUnauthorized, "unauthorized")
	f.Do("POST", f.path("/tasks/999/approve"), map[string]any{"checked": manual}, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("POST", f.path(fmt.Sprintf("/tasks/%d/approve", f.taskID(1))), "not json", f.ada).Expect(http.StatusBadRequest, "bad_request")
	if got := f.merges(); got != 0 {
		t.Fatalf("merge requests = %d, want none", got)
	}
}

func TestApproveTwiceAtOnceMergesOnce(t *testing.T) {
	f, n := inUserCheck(t)
	manual := f.manual()
	bob, _, _ := f.member("bob", "push")
	path := f.path(fmt.Sprintf("/tasks/%d/approve", f.taskID(1)))
	// Both approvals have read GitHub before either takes the project's lock.
	next := f.Config.GitHub.HTTP.Transport
	arrived, release := make(chan struct{}), make(chan struct{})
	f.Config.GitHub.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/graphql" {
			arrived <- struct{}{}
			<-release
		}
		return next.RoundTrip(r)
	})}
	answers := make(chan *apptest.Response, 2)
	for _, session := range []*http.Cookie{f.ada, bob} {
		go func() { answers <- f.Do("POST", path, map[string]any{"checked": manual}, session) }()
	}
	<-arrived
	<-arrived
	close(release)
	first, second := <-answers, <-answers
	if first.Status != http.StatusOK {
		first, second = second, first
	}
	first.Expect(http.StatusOK, "")
	second.Expect(http.StatusConflict, "pull_request_closed")
	if got := f.merges(); got != 1 {
		t.Fatalf("merge requests = %d, want one", got)
	}
	if got := f.approvals(); got != 1 {
		t.Fatalf("approved runs = %d, want one", got)
	}
	if pr := f.repo.Issue(n); pr.State != "closed" || f.task(1).Column != "closed" {
		t.Fatalf("pull request = %s, task in %s; want both closed", pr.State, f.task(1).Column)
	}
}
