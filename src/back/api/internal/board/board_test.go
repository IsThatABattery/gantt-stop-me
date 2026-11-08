package board_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
)

// fixture is octo/hello added as a project by ada, who can push to it.
type fixture struct {
	*apptest.Env
	t       *testing.T
	ada     *http.Cookie
	token   string // ada's GitHub token
	repo    *githubtest.Repo
	project int64
}

// setup makes a fixture whose repository had an open issue for each title, numbered from 1, when it was added.
func setup(t *testing.T, titles ...string) *fixture {
	t.Helper()
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	token := env.LinkGitHub(ada, "ada-gh")
	repo.Grant(token, "push")
	for _, title := range titles {
		repo.AddIssue(title)
	}
	return &fixture{Env: env, t: t, ada: ada, token: token, repo: repo, project: env.AddProject(ada, "octo", "hello")}
}

// path is the URL path of the project's route suffix, such as "/board".
func (f *fixture) path(suffix string, args ...any) string {
	return fmt.Sprintf("/api/projects/%d", f.project) + fmt.Sprintf(suffix, args...)
}

// reader signs up bob, who can only read the repository, and returns his session.
func (f *fixture) reader() *http.Cookie {
	f.t.Helper()
	bob := f.SignUp("bob@example.com", "correct horse")
	f.repo.Grant(f.LinkGitHub(bob, "bob-gh"), "pull")
	return bob
}

// stranger signs up eve, whose GitHub account cannot see the repository, and returns her session.
func (f *fixture) stranger() *http.Cookie {
	f.t.Helper()
	eve := f.SignUp("eve@example.com", "correct horse")
	f.LinkGitHub(eve, "eve-gh")
	return eve
}

type task struct {
	ID, ColumnID              int64
	IssueNumber, Version      int
	Position                  float64
	Title, State, URL         string
	RunStatus, NeedsAttention *string
}

// runs gives issue number's task test runs with these statuses, oldest first, and needs_attention, "" for none.
func (f *fixture) runs(number int, needsAttention string, statuses ...string) {
	f.t.Helper()
	for _, s := range statuses {
		f.exec(`insert into test_runs (task_id, status) select id, $2 from tasks where issue_number = $1`, number, s)
	}
	f.exec(`update tasks set needs_attention = nullif($2, '') where issue_number = $1`, number, needsAttention)
}

// orNull is s, or "null" for nil, to compare the API's nullable strings.
func orNull(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

type column struct {
	ID          int64
	Key, Name   string
	Position    int
	ClosesIssue bool
}

type board struct {
	Project struct {
		ID                      int64
		Owner, Name, Permission string
	}
	Columns []column
	Tasks   []task
}

// board loads the board as ada.
func (f *fixture) board() board {
	f.t.Helper()
	res := f.Do("GET", f.path("/board"), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var b board
	res.JSON(&b)
	return b
}

// column returns the id of the project's column with this key.
func (f *fixture) column(key string) int64 {
	f.t.Helper()
	var id int64
	if err := f.DB.QueryRow(context.Background(), `select id from columns where project_id = $1 and key = $2`, f.project, key).Scan(&id); err != nil {
		f.t.Fatalf("column %s: %v", key, err)
	}
	return id
}

type storedTask struct {
	Column   string // key
	Position float64
	Version  int
	Title    string
	State    string
}

// task returns the stored task of issue number.
func (f *fixture) task(number int) (int64, storedTask) {
	f.t.Helper()
	var id int64
	var s storedTask
	err := f.DB.QueryRow(context.Background(), `
		select t.id, c.key, t.position, t.version, t.title, t.state from tasks t join columns c on c.id = t.column_id
		where t.project_id = $1 and t.issue_number = $2`, f.project, number).Scan(&id, &s.Column, &s.Position, &s.Version, &s.Title, &s.State)
	if err != nil {
		f.t.Fatalf("task of issue %d: %v", number, err)
	}
	return id, s
}

type label struct{ Name, Color string }

// details returns the body and labels of issue number's task as GET task answers them to ada, failing the test
// unless the row stores the same.
func (f *fixture) details(number int) (string, []label) {
	f.t.Helper()
	id, _ := f.task(number)
	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var got struct {
		Body   string
		Labels []label
	}
	res.JSON(&got)
	var body string
	var labels []label
	if err := f.DB.QueryRow(context.Background(), `select body, labels from tasks where id = $1`, id).Scan(&body, &labels); err != nil {
		f.t.Fatal(err)
	}
	if got.Labels == nil || got.Body != body || !slices.Equal(got.Labels, labels) {
		f.t.Fatalf("GET task answers %q %v, the row stores %q %v", got.Body, got.Labels, body, labels)
	}
	return body, labels
}

// order returns the issue numbers of the column's tasks, top to bottom.
func (f *fixture) order(key string) []int {
	f.t.Helper()
	var numbers []int
	for _, t := range f.board().Tasks {
		if t.ColumnID == f.column(key) {
			numbers = append(numbers, t.IssueNumber)
		}
	}
	return numbers
}

// exec runs SQL to set up a test, failing it on error.
func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.DB.Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatal(err)
	}
}

// dueSync makes the next board load sync, as if the last sync was a minute ago.
func (f *fixture) dueSync() {
	f.t.Helper()
	f.exec(`update projects set synced_at = now() - interval '1 minute' where id = $1`, f.project)
}

// since returns the GitHub requests made after the first n.
func (f *fixture) since(n int) []string { return f.GitHub.Requests()[n:] }

type roundTrip func(*http.Request) (*http.Response, error)

func (rt roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return rt(r) }

// intercept sends the API's later requests to GitHub through fn, which may pass them on to next.
func (f *fixture) intercept(fn func(r *http.Request, next http.RoundTripper) (*http.Response, error)) {
	next := f.Config.GitHub.HTTP.Transport
	f.Config.GitHub.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) { return fn(r, next) })}
}
