package automation_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
)

// requestChanges asks, as session, for changes to issue 1's task with feedback.
func (f *fixture) requestChanges(session *http.Cookie, feedback string) *apptest.Response {
	f.t.Helper()
	return f.Do("POST", f.path(fmt.Sprintf("/tasks/%d/request-changes", f.taskID(1))), map[string]any{"feedback": feedback}, session)
}

// shownTask is a task as Get task answers it, with what these tests check.
type shownTask struct {
	ID           int64
	PullRequests []struct {
		Number int
	}
	Assignees   []struct{ Login string }
	TakenOverBy *struct {
		ID    int64
		Login *string
	}
}

func TestRequestChanges(t *testing.T) {
	f, n := inUserCheck(t)
	bob, _, _ := f.member("bob", "push")
	res := f.requestChanges(bob, "  The bars overlap on narrow screens.\n")
	res.Expect(http.StatusOK, "")
	var got shownTask
	res.JSON(&got)
	if got.ID != f.taskID(1) || len(got.PullRequests) != 1 || got.PullRequests[0].Number != n {
		t.Fatalf("answer = %+v, want the task with its pull request", got)
	}
	if s := f.task(1); s.Column != "in-progress" {
		t.Fatalf("task = %+v, want it back in In Progress", s)
	}
	pr := f.repo.Issue(n)
	if last := pr.Comments[len(pr.Comments)-1]; last.Author != "bob" || last.Body != "Changes requested by bob:\n\nThe bars overlap on narrow screens." {
		t.Fatalf("last comment = %+v, want bob's feedback, trimmed and signed", last)
	}
	if !pr.Pull.Draft || !f.record(n).Draft {
		t.Fatal("the pull request is not a draft again")
	}

	// Marking it ready again, as after a failed run, tests it again.
	runs := len(f.runs())
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft, p.HeadSHA = false, "a2" })
	f.poll()
	if all := f.runs(); len(all) != runs+1 || all[runs].Status != "queued" || all[runs].Head != "a2" || f.task(1).Column != "testing" {
		t.Fatalf("after ready again: runs %+v, task %+v; want a new run of a2 in Testing", all, f.task(1))
	}
}

func TestRequestChangesNeedsUserCheck(t *testing.T) {
	f := on(t) // the task is in Backlog
	f.requestChanges(f.ada, "Fix it.").Expect(http.StatusConflict, "not_in_user_check")
}

func TestRequestChangesNeedsAnOpenPullRequest(t *testing.T) {
	f, n := inUserCheck(t)
	f.repo.EditIssue(n, func(is *githubtest.Issue) { is.State = "closed" })
	comments := len(f.repo.Issue(n).Comments)
	f.requestChanges(f.ada, "Fix it.").Expect(http.StatusConflict, "no_pull_request")
	if f.task(1).Column != "user-check" || len(f.repo.Issue(n).Comments) != comments {
		t.Fatal("a refused request changed the task or commented")
	}
}

func TestRequestChangesFeedback(t *testing.T) {
	f, _ := inUserCheck(t)
	for _, bad := range []string{"", " \n ", strings.Repeat("a", 10001)} {
		f.requestChanges(f.ada, bad).Expect(http.StatusBadRequest, "invalid_feedback")
	}
	f.requestChanges(f.ada, strings.Repeat("é", 10000)).Expect(http.StatusOK, "")
}

func TestRequestChangesNeedsWriteAccess(t *testing.T) {
	f, _ := inUserCheck(t)
	reader, _, _ := f.member("bob", "pull")
	stranger, _, _ := f.member("eve", "")
	f.requestChanges(reader, "Fix it.").Expect(http.StatusForbidden, "forbidden")
	f.requestChanges(stranger, "Fix it.").Expect(http.StatusNotFound, "not_found")
	f.Do("POST", f.path(fmt.Sprintf("/tasks/%d/request-changes", f.taskID(1))), map[string]any{"feedback": "Fix it."}).
		Expect(http.StatusUnauthorized, "unauthorized")
	f.Do("POST", f.path("/tasks/999/request-changes"), map[string]any{"feedback": "Fix it."}, f.ada).Expect(http.StatusNotFound, "not_found")
}

// changeRequests counts the "Changes requested" comments on pull request n.
func (f *fixture) changeRequests(n int) int {
	count := 0
	for _, c := range f.repo.Issue(n).Comments {
		if strings.HasPrefix(c.Body, "Changes requested by ") {
			count++
		}
	}
	return count
}

// backToUserCheck puts issue 1's task back in User Check, as if it had been tested again.
func (f *fixture) backToUserCheck() {
	f.exec(`update tasks set column_id = (select id from columns c where c.project_id = tasks.project_id and c.key = 'user-check')`)
}

func TestRequestChangesPrefersTheTestedPullRequest(t *testing.T) {
	f, n := inUserCheck(t)
	draft := f.repo.AddPull("Draft", githubtest.Pull{Draft: true, Closes: f.closing(1)})
	ready := f.repo.AddPull("Ready", githubtest.Pull{Closes: f.closing(1)})
	f.requestChanges(f.ada, "Fix it.").Expect(http.StatusOK, "")
	if f.changeRequests(n) != 1 || f.changeRequests(draft) != 0 || f.changeRequests(ready) != 0 {
		t.Fatal("the feedback did not go to the tested pull request alone")
	}

	// Once that one is closed, an open one ready for review wins over a draft GitHub lists first.
	f.repo.EditIssue(n, func(is *githubtest.Issue) { is.State = "closed" })
	f.exec(`update tasks set pull_requests_at = null`)
	f.backToUserCheck()
	f.requestChanges(f.ada, "Fix it.").Expect(http.StatusOK, "")
	if f.changeRequests(ready) != 1 || f.changeRequests(draft) != 0 || !f.repo.Issue(ready).Pull.Draft {
		t.Fatal("the feedback did not go to the ready pull request, or it is not a draft now")
	}
}

// A pull request of another repository can close the issue too, and its number is another pull request's here.
func TestRequestChangesIgnoresOtherRepositories(t *testing.T) {
	f, n := inUserCheck(t)
	f.repo.EditIssue(n, func(is *githubtest.Issue) { is.State = "closed" })
	other := f.GitHub.AddRepo("octo", "fork")
	other.Grant(f.token, "push")
	other.AddIssue("Padding")
	if m := other.AddPull("Elsewhere", githubtest.Pull{Closes: f.closing(1)}); m != n {
		t.Fatalf("the other repository's pull request is #%d, want #%d like this one's", m, n)
	}
	f.requestChanges(f.ada, "Fix it.").Expect(http.StatusConflict, "no_pull_request")
	if f.changeRequests(n) != 0 || f.changeRequests(1) != 0 || f.repo.Issue(n).Pull.Draft {
		t.Fatal("request changes commented by another repository's pull request's number")
	}
}

// The comment is the last write, so a move GitHub refused, then a retry, comments once.
func TestRequestChangesRetriedCommentsOnce(t *testing.T) {
	f, n := inUserCheck(t)
	f.GitHub.Fail("POST /repositories/{id}/issues/{number}/labels", http.StatusBadGateway)
	f.requestChanges(f.ada, "Fix it.").Expect(http.StatusBadGateway, "github_error")
	if f.task(1).Column != "user-check" || f.changeRequests(n) != 0 {
		t.Fatalf("after a failed move: task %+v, %d comments; want it in User Check with none", f.task(1), f.changeRequests(n))
	}
	f.GitHub.Fail("POST /repositories/{id}/issues/{number}/labels", 0)
	f.requestChanges(f.ada, "Fix it.").Expect(http.StatusOK, "")
	if got := f.changeRequests(n); got != 1 || f.task(1).Column != "in-progress" {
		t.Fatalf("after the retry: %d comments, task %+v; want one, in In Progress", got, f.task(1))
	}
}

func TestRequestChangesWhenTheDraftFails(t *testing.T) {
	f, n := inUserCheck(t)
	f.GitHub.Fail("GET /repositories/{id}/pulls/{number}", http.StatusBadGateway)
	res := f.requestChanges(f.ada, "Fix it.")
	res.Expect(http.StatusBadGateway, "github_error")
	if msg := errorMessage(res); !strings.Contains(msg, "draft") {
		t.Fatalf("message = %q, want it to say the pull request is not a draft", msg)
	}
	if f.task(1).Column != "in-progress" || f.changeRequests(n) != 1 || f.repo.Issue(n).Pull.Draft {
		t.Fatalf("task %+v, %d comments; want it in In Progress with the feedback, the pull request still ready", f.task(1), f.changeRequests(n))
	}
}
