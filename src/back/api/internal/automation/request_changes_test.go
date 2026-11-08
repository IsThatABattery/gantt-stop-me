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
	ID, ColumnID int64
	PullRequests []struct {
		Number int
		State  string
		Draft  bool
	}
	Assignees []struct{ Login string }
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
