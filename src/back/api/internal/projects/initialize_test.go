package projects_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
)

type storedColumn struct {
	Key, Name   string
	Position    int
	ClosesIssue bool
}

type storedTask struct {
	Column string // key
	Title  string
	State  string
}

func storedColumns(t *testing.T, env *apptest.Env) []storedColumn {
	t.Helper()
	rows, _ := env.DB.Query(context.Background(), `select key, name, position, closes_issue from columns order by position`)
	cols, err := pgx.CollectRows(rows, pgx.RowToStructByPos[storedColumn])
	if err != nil {
		t.Fatal(err)
	}
	return cols
}

// storedTasks returns every task in board order.
func storedTasks(t *testing.T, env *apptest.Env) []storedTask {
	t.Helper()
	rows, _ := env.DB.Query(context.Background(), `
		select c.key, t.title, t.state from tasks t join columns c on c.id = t.column_id order by c.position, t.position`)
	tasks, err := pgx.CollectRows(rows, pgx.RowToStructByPos[storedTask])
	if err != nil {
		t.Fatal(err)
	}
	return tasks
}

var wantLabels = []string{"board:backlog", "board:in-progress", "board:testing", "board:user-check", "board:closed"}

func TestInitializeCreatesColumnsAndLabels(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	repo.AddIssue("Labeled before", "bug", "board:testing") // so board:testing and bug already exist

	env.AddProject(ada, "octo", "hello")
	want := []storedColumn{
		{"backlog", "Backlog", 0, false},
		{"in-progress", "In Progress", 1, false},
		{"testing", "Testing", 2, false},
		{"user-check", "User Check", 3, false},
		{"closed", "Closed", 4, true},
	}
	if got := storedColumns(t, env); !slices.Equal(got, want) {
		t.Fatalf("columns = %+v, want %+v", got, want)
	}
	labels := repo.Labels()
	for _, l := range wantLabels {
		if !slices.Contains(labels, l) {
			t.Errorf("label %s missing from %v", l, labels)
		}
	}
	if len(labels) != len(wantLabels)+1 {
		t.Fatalf("labels = %v, want each board label once, plus bug", labels)
	}
}

func TestInitializeImportsOpenIssues(t *testing.T) {
	env := apptest.New(t)
	env.GitHub.PageSize = 2 // so the import walks pages
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	repo.AddIssue("One")
	repo.AddPullRequest("A pull request")
	repo.AddIssue("Two", "bug", "board:user-check")
	closed := repo.AddIssue("Closed already")
	repo.EditIssue(closed, func(is *githubtest.Issue) { is.State = "closed" })
	repo.AddIssue("Labeled closed but open", "board:closed")
	repo.AddIssue("Three", "Board:Testing", "board:in-progress") // the first label in column order wins, in any case
	repo.AddIssue("Four")

	env.AddProject(ada, "octo", "hello")
	want := []storedTask{
		{"backlog", "One", "open"},
		{"backlog", "Labeled closed but open", "open"},
		{"backlog", "Four", "open"},
		{"in-progress", "Three", "open"},
		{"user-check", "Two", "open"},
	}
	if got := storedTasks(t, env); !slices.Equal(got, want) {
		t.Fatalf("tasks = %+v\nwant %+v", got, want)
	}
	pages := 0
	for _, r := range env.GitHub.Requests() {
		if strings.HasPrefix(r, "GET /repositories/1/issues?") {
			pages++
		}
	}
	if pages != 3 {
		t.Fatalf("import read %d pages, want 3 (6 open issues and pull requests, 2 a page)", pages)
	}
}

// slowIssues delays each issue list request to GitHub and records when the last one was answered.
type slowIssues struct {
	http.RoundTripper
	answered *atomic.Int64 // unix nanoseconds
}

func (s slowIssues) RoundTrip(r *http.Request) (*http.Response, error) {
	if !strings.HasSuffix(r.URL.Path, "/issues") {
		return s.RoundTripper.RoundTrip(r)
	}
	time.Sleep(300 * time.Millisecond)
	res, err := s.RoundTripper.RoundTrip(r)
	s.answered.Store(time.Now().UnixNano())
	return res, err
}

func TestInitializeRecordsTheSyncFromBeforeTheFetch(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	repo.AddIssue("One")
	var answered atomic.Int64
	env.Config.GitHub.HTTP = &http.Client{Transport: slowIssues{env.Config.GitHub.HTTP.Transport, &answered}}

	env.AddProject(ada, "octo", "hello")
	var synced time.Time
	if err := env.DB.QueryRow(context.Background(), `select synced_at from projects`).Scan(&synced); err != nil {
		t.Fatal(err)
	}
	// The first sync re-reads from synced_at on, so an issue changed while the fetch ran must fall after it.
	if fetched := time.Unix(0, answered.Load()); !synced.Before(fetched) {
		t.Fatalf("synced_at = %v, want it before the issue list was answered at %v", synced, fetched)
	}
}

func TestInitializeImportsBodyAndLabels(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	repo.AddIssue("Detailed", "bug", "board:testing")
	repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "The details." })
	repo.AddIssue("Bare") // GitHub sends its body as null and no labels

	project := env.AddProject(ada, "octo", "hello")
	type label struct{ Name, Color string }
	for _, c := range []struct {
		number int
		body   string
		labels []label
	}{
		{1, "The details.", []label{{"bug", "ededed"}, {"board:testing", "ededed"}}},
		{2, "", []label{}},
	} {
		var id int64
		var body string
		var labels []label
		err := env.DB.QueryRow(context.Background(), `select id, body, labels from tasks where issue_number = $1`, c.number).Scan(&id, &body, &labels)
		if err != nil {
			t.Fatal(err)
		}
		res := env.Do("GET", fmt.Sprintf("/api/projects/%d/tasks/%d", project, id), nil, ada)
		res.Expect(http.StatusOK, "")
		var got struct {
			Body   string
			Labels []label
		}
		res.JSON(&got)
		if body != c.body || !slices.Equal(labels, c.labels) || got.Body != c.body || got.Labels == nil || !slices.Equal(got.Labels, c.labels) {
			t.Errorf("issue %d: stored %q %v, GET task %q %v; want %q %v", c.number, body, labels, got.Body, got.Labels, c.body, c.labels)
		}
	}
}

// GitHub retires a token as it refreshes it, so another request's refresh can beat setup's calls: adding again
// finishes it.
func TestInitializeAfterAnotherRequestRefreshedTheToken(t *testing.T) {
	for _, route := range []string{"POST /repositories/{id}/labels", "GET /repositories/{id}/issues"} {
		t.Run(route, func(t *testing.T) {
			env := apptest.New(t)
			ada := env.SignUp("ada@example.com", "correct horse")
			expiringLink(t, env, ada, time.Hour)
			refreshed := env.RefreshDuring(route, ada)
			add(env, ada, "https://github.com/octo/hello").Expect(http.StatusBadGateway, "github_error")
			refreshed()
			add(env, ada, "https://github.com/octo/hello").Expect(http.StatusOK, "") // set up now, as the project added before
		})
	}
}

func TestInitializeFinishesHalfInitializedProject(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	repo.AddIssue("One")
	env.GitHub.Fail("GET /repositories/{id}/issues", http.StatusInternalServerError)

	add(env, ada, "https://github.com/octo/hello").Expect(http.StatusBadGateway, "github_error")
	var id int64
	var synced bool
	env.DB.QueryRow(context.Background(), `select id, synced_at is not null from projects`).Scan(&id, &synced)
	if id == 0 || synced {
		t.Fatalf("after a failed import: project %d, initialized %v; want it stored and not initialized", id, synced)
	}
	if n := len(storedColumns(t, env)); n != 5 {
		t.Fatalf("%d columns, want 5 made before the failure", n)
	}
	env.Do("GET", fmt.Sprintf("/api/projects/%d/board", id), nil, ada).Expect(http.StatusNotFound, "not_found")

	env.GitHub.Fail("GET /repositories/{id}/issues", 0)
	res := add(env, ada, "https://github.com/octo/hello")
	res.Expect(http.StatusOK, "") // not 201: the first attempt created the project
	var p project
	res.JSON(&p)
	if p.ID != id {
		t.Fatalf("finished project %d, want %d", p.ID, id)
	}
	if n := len(storedColumns(t, env)); n != 5 {
		t.Fatalf("%d columns, want 5", n)
	}
	if n := len(repo.Labels()); n != len(wantLabels) {
		t.Fatalf("labels = %v, want each board label once", repo.Labels())
	}
	if got := storedTasks(t, env); !slices.Equal(got, []storedTask{{"backlog", "One", "open"}}) {
		t.Fatalf("tasks = %+v", got)
	}
	env.Do("GET", fmt.Sprintf("/api/projects/%d/board", id), nil, ada).Expect(http.StatusOK, "")
}
