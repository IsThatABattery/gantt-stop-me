package board_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/github/githubtest"
)

type taskDetails struct {
	task
	Body   string
	Labels []label
}

func TestGetTask(t *testing.T) {
	f := setup(t, "One")
	// pull_requests_at: its linked pull requests read just now, which read the issue's id from GitHub.
	f.exec(`update tasks set body = 'The details.', labels = '[{"name": "bug", "color": "d73a4a"}, {"name": "board:backlog", "color": "c5def5"}]',
		pull_requests_at = now()`)
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "Only on GitHub" }) // no sync is due, so the row answers
	id, _ := f.task(1)

	n := len(f.GitHub.Requests())
	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.reader())
	res.Expect(http.StatusOK, "")
	var got taskDetails
	res.JSON(&got)
	if got.ID != id || got.IssueNumber != 1 || got.Title != "One" || got.State != "open" || got.ColumnID != f.column("backlog") || got.URL == "" {
		t.Fatalf("task = %+v", got.task)
	}
	if want := []label{{"bug", "d73a4a"}, {"board:backlog", "c5def5"}}; got.Body != "The details." || !slices.Equal(got.Labels, want) {
		t.Fatalf("body %q, labels %+v; want the stored ones", got.Body, got.Labels)
	}
	for _, r := range f.since(n) {
		if strings.HasPrefix(r, "GET /repositories/1/issues/") {
			t.Fatalf("GET task fetched the issue from GitHub: %v", f.since(n))
		}
	}
}

func TestGetTaskRunStatus(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	get := func() task {
		res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
		res.Expect(http.StatusOK, "")
		var got task
		res.JSON(&got)
		return got
	}
	if got := get(); got.RunStatus != nil || got.NeedsAttention != nil {
		t.Fatalf("untested task = %+v, want null run status and needs attention", got)
	}
	f.runs(1, "", "error", "passed")
	if got := get(); orNull(got.RunStatus) != "passed" || got.NeedsAttention != nil {
		t.Fatalf("task = %+v, want the latest run's status", got)
	}
}

func TestGetTaskSyncsWhenDue(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.repo.EditIssue(1, func(is *githubtest.Issue) {
		is.Title, is.State, is.Body, is.Labels = "Renamed", "closed", "New body", []string{"bug"}
	})
	get := func() taskDetails {
		t.Helper()
		res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
		res.Expect(http.StatusOK, "")
		var got taskDetails
		res.JSON(&got)
		return got
	}

	if got := get(); got.Title != "One" || got.State != "open" || got.Body != "" || len(got.Labels) != 0 {
		t.Fatalf("within 30 seconds of the last sync: %+v, want the cached task", got)
	}
	f.dueSync()
	if got := get(); got.Title != "Renamed" || got.State != "closed" || got.ColumnID != f.column("closed") || got.Body != "New body" ||
		!slices.Equal(got.Labels, []label{{"bug", "ededed"}}) {
		t.Fatalf("after a due sync: %+v, want the issue as GitHub has it", got)
	}
}

func TestGetTaskHidesLabelIDs(t *testing.T) {
	f := setup(t, "One")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Labels = []string{"bug"} })
	f.dueSync()
	f.board()
	id, _ := f.task(1)
	var stored bool
	f.DB.QueryRow(t.Context(), `select labels->0->'id' is not null from tasks where id = $1`, id).Scan(&stored)
	if !stored {
		t.Fatal("the sync stored the label without its id")
	}

	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var got struct{ Labels []map[string]any }
	res.JSON(&got)
	if len(got.Labels) != 1 || len(got.Labels[0]) != 2 || got.Labels[0]["name"] != "bug" || got.Labels[0]["color"] != "ededed" {
		t.Fatalf("labels = %v, want only the name and color of bug", got.Labels)
	}
}

func TestGetTaskUnknown(t *testing.T) {
	f := setup(t, "One")
	f.Do("GET", f.path("/tasks/999"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path("/tasks/nope"), nil, f.ada).Expect(http.StatusNotFound, "not_found")

	// A task of another project is not found through this one.
	other := f.GitHub.AddRepo("octo", "other")
	other.Grant(f.token, "push")
	other.AddIssue("Elsewhere")
	otherID := f.AddProject(f.ada, "octo", "other")
	var foreign int64
	f.DB.QueryRow(t.Context(), `select id from tasks where project_id = $1`, otherID).Scan(&foreign)
	f.Do("GET", f.path("/tasks/%d", foreign), nil, f.ada).Expect(http.StatusNotFound, "not_found")
}

func TestGetTaskNeedsAccess(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.Do("GET", f.path("/tasks/%d", id), nil, f.stranger()).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path("/tasks/%d", id), nil).Expect(http.StatusUnauthorized, "unauthorized")
}

func TestGetTaskWithRevokedCredential(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.dueSync()
	f.GitHub.Fail("GET /repositories/{id}/issues", http.StatusUnauthorized) // the token expired or was revoked
	f.Do("GET", f.path("/tasks/%d", id), nil, f.ada).Expect(http.StatusForbidden, "github_not_linked")
}

func TestGetTaskWhenSyncFails(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "Not fetched" })
	f.dueSync()
	f.GitHub.Fail("GET /repositories/{id}/issues", http.StatusInternalServerError)
	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var got taskDetails
	res.JSON(&got)
	if got.ID != id || got.Body != "" {
		t.Fatalf("task = %+v, want the cached one", got)
	}
}

type assignee struct{ Login, AvatarURL string }

type pullRequest struct {
	Number       int
	Title, State string
	Draft        bool
	URL          string
}

// The project's automation is off: the pull requests come from GitHub, not from automation's records.
func TestGetTaskAssigneesAndPullRequests(t *testing.T) {
	f := setup(t, "One", "Two")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Assignees = []string{"ada-gh", "bob-gh"} })
	closes := []githubtest.Ref{{RepoID: f.repo.ID, Number: 1}}
	open := f.repo.AddPull("Open fix", githubtest.Pull{Closes: closes})
	draft := f.repo.AddPull("Draft fix", githubtest.Pull{Closes: closes, Draft: true})
	closed := f.repo.AddPull("Abandoned fix", githubtest.Pull{Closes: closes})
	f.repo.EditIssue(closed, func(is *githubtest.Issue) { is.State = "closed" })
	merged := f.repo.AddPull("Merged fix", githubtest.Pull{Closes: closes, MergedWith: "squash"})
	f.repo.EditIssue(merged, func(is *githubtest.Issue) { is.State = "closed" })
	f.repo.AddPull("Other fix", githubtest.Pull{Closes: []githubtest.Ref{{RepoID: f.repo.ID, Number: 2}}})
	f.dueSync()
	id, _ := f.task(1)

	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var got struct {
		Assignees    []assignee
		PullRequests []pullRequest
		TakenOverBy  *struct{}
	}
	res.JSON(&got)
	wantAssignees := []assignee{{"ada-gh", "https://avatars.example.com/ada-gh"}, {"bob-gh", "https://avatars.example.com/bob-gh"}}
	if !slices.Equal(got.Assignees, wantAssignees) {
		t.Fatalf("assignees = %+v, want %+v", got.Assignees, wantAssignees)
	}
	pr := func(n int, title, state string, draft bool) pullRequest {
		return pullRequest{n, title, state, draft, fmt.Sprintf("%s/octo/hello/pull/%d", f.GitHub.Server.URL, n)}
	}
	want := []pullRequest{pr(open, "Open fix", "open", false), pr(draft, "Draft fix", "open", true),
		pr(closed, "Abandoned fix", "closed", false), pr(merged, "Merged fix", "merged", false)}
	if !slices.Equal(got.PullRequests, want) {
		t.Fatalf("pull requests = %+v, want %+v", got.PullRequests, want)
	}
	if got.TakenOverBy != nil {
		t.Fatal("takenOverBy is set on a task nobody took over")
	}
	for _, task := range f.boardAssignees() {
		if task.IssueNumber == 1 && !slices.Equal(task.Assignees, wantAssignees) || task.IssueNumber == 2 && len(task.Assignees) != 0 {
			t.Fatalf("board task %d has assignees %+v", task.IssueNumber, task.Assignees)
		}
	}

	// The next sync overwrites the assignees, as it does the other cached fields.
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Assignees = nil })
	f.dueSync()
	for _, task := range f.boardAssignees() {
		if len(task.Assignees) != 0 {
			t.Fatalf("board task %d has assignees %+v after GitHub dropped them", task.IssueNumber, task.Assignees)
		}
	}
}

// boardAssignees loads the board as ada, with each task's issue number and assignees.
func (f *fixture) boardAssignees() []struct {
	IssueNumber int
	Assignees   []assignee
} {
	f.t.Helper()
	res := f.Do("GET", f.path("/board"), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var b struct {
		Tasks []struct {
			IssueNumber int
			Assignees   []assignee
		}
	}
	res.JSON(&b)
	return b.Tasks
}

func TestGetTaskReadsPullRequestsAtMostEvery30Seconds(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	get := func() []pullRequest {
		t.Helper()
		res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
		res.Expect(http.StatusOK, "")
		var got struct{ PullRequests []pullRequest }
		res.JSON(&got)
		return got.PullRequests
	}
	if got := get(); got == nil || len(got) != 0 {
		t.Fatalf("pull requests = %#v, want []", got)
	}
	f.repo.AddPull("Fix", githubtest.Pull{Closes: []githubtest.Ref{{RepoID: f.repo.ID, Number: 1}}})
	if got := get(); len(got) != 0 {
		t.Fatalf("within 30 seconds: pull requests = %+v, want the cached none", got)
	}
	f.exec(`update tasks set pull_requests_at = now() - interval '31 seconds'`)
	if got := get(); len(got) != 1 || got[0].Title != "Fix" {
		t.Fatalf("after 30 seconds: pull requests = %+v, want the new one", got)
	}

	// The issue's GraphQL id is read once, by the sync or else the first read of the pull requests.
	f.exec(`update tasks set issue_node_id = null`)
	n := len(f.GitHub.Requests())
	for range 2 {
		f.exec(`update tasks set pull_requests_at = null`)
		get()
	}
	if got := slices.DeleteFunc(f.since(n), func(r string) bool { return r != "GET /repositories/1/issues/1" }); len(got) != 1 {
		t.Fatalf("issue reads = %v, want one", got)
	}
}

// A failed read answers the cached pull requests, saying why, and gives its claim back so the next view tries again.
func TestGetTaskWhenPullRequestsFail(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.repo.AddPull("Fix", githubtest.Pull{Closes: []githubtest.Ref{{RepoID: f.repo.ID, Number: 1}}})
	type answer struct {
		PullRequests      []pullRequest
		PullRequestsError *string
	}
	get := func() answer {
		t.Helper()
		res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
		res.Expect(http.StatusOK, "")
		var got answer
		res.JSON(&got)
		return got
	}
	if got := get(); len(got.PullRequests) != 1 || got.PullRequestsError != nil {
		t.Fatalf("got %+v, want the pull request and no error", got)
	}
	f.repo.AddPull("Second fix", githubtest.Pull{Closes: []githubtest.Ref{{RepoID: f.repo.ID, Number: 1}}})
	f.exec(`update tasks set pull_requests_at = now() - interval '31 seconds'`)
	f.GitHub.Fail("POST /graphql", http.StatusBadGateway)
	if got := get(); len(got.PullRequests) != 1 || got.PullRequestsError == nil {
		t.Fatalf("got %+v, want the cached pull request and an error", got)
	}
	f.GitHub.Fail("POST /graphql", 0)
	if got := get(); len(got.PullRequests) != 2 || got.PullRequestsError != nil {
		t.Fatalf("got %+v, want both pull requests read again", got)
	}
}

// A pull request of another repository can close the issue too. It is left out, so a member who cannot see that
// repository never sees it through another's cached read, nor its number taken for one here.
func TestGetTaskLeavesOutOtherRepositoriesPullRequests(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	private := f.GitHub.AddRepo("octo", "private")
	private.Grant(f.token, "push") // ada sees it, bob does not
	private.AddPull("Secret fix", githubtest.Pull{Closes: []githubtest.Ref{{RepoID: f.repo.ID, Number: 1}}})
	reader := f.reader()
	for _, session := range []*http.Cookie{f.ada, reader} { // ada's read fills the cache bob's answers from
		res := f.Do("GET", f.path("/tasks/%d", id), nil, session)
		res.Expect(http.StatusOK, "")
		var got struct{ PullRequests []pullRequest }
		res.JSON(&got)
		if len(got.PullRequests) != 0 {
			t.Fatalf("pull requests = %+v, want none of another repository", got.PullRequests)
		}
	}
}
