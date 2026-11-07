package board_test

import (
	"net/http"
	"slices"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
)

// move moves the task of issue number into the column with key, before the task of issue before (0 for the end),
// with the task's current version.
func (f *fixture) move(session *http.Cookie, number int, key string, before int) *apptest.Response {
	f.t.Helper()
	id, s := f.task(number)
	body := map[string]any{"columnId": f.column(key), "beforeTaskId": nil, "version": s.Version}
	if before != 0 {
		body["beforeTaskId"], _ = f.task(before)
	}
	return f.Do("POST", f.path("/tasks/%d/move", id), body, session)
}

func TestMoveAcrossColumns(t *testing.T) {
	f := setup(t, "One", "Two")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Labels = []string{"bug", "board:backlog"} })

	res := f.move(f.ada, 1, "testing", 0)
	res.Expect(http.StatusOK, "")
	var got task
	res.JSON(&got)
	if got.ColumnID != f.column("testing") || got.Version != 2 || got.Position != 1 || got.State != "open" {
		t.Fatalf("task = %+v", got)
	}
	if is := f.repo.Issue(1); !slices.Equal(is.Labels, []string{"bug", "board:testing"}) || is.State != "open" {
		t.Fatalf("issue = %+v, want board:backlog swapped for board:testing", is)
	}
	if _, s := f.task(1); s.Column != "testing" || s.Version != 2 {
		t.Fatalf("stored task = %+v", s)
	}

	// An issue missing its old label, like Two, still moves.
	f.move(f.ada, 2, "testing", 1).Expect(http.StatusOK, "")
	if got := f.order("testing"); !slices.Equal(got, []int{2, 1}) {
		t.Fatalf("testing = %v, want 2 placed before 1", got)
	}
}

func TestMoveIntoAndOutOfClosed(t *testing.T) {
	f := setup(t, "One")
	f.move(f.ada, 1, "closed", 0).Expect(http.StatusOK, "")
	if is := f.repo.Issue(1); is.State != "closed" || !slices.Equal(is.Labels, []string{"board:closed"}) {
		t.Fatalf("issue after moving into Closed = %+v", is)
	}
	if _, s := f.task(1); s.State != "closed" {
		t.Fatalf("stored task = %+v", s)
	}

	f.move(f.ada, 1, "user-check", 0).Expect(http.StatusOK, "")
	if is := f.repo.Issue(1); is.State != "open" || !slices.Equal(is.Labels, []string{"board:user-check"}) {
		t.Fatalf("issue after moving out of Closed = %+v", is)
	}
	if _, s := f.task(1); s.State != "open" || s.Column != "user-check" || s.Version != 3 {
		t.Fatalf("stored task = %+v", s)
	}
}

func TestMoveWithinColumn(t *testing.T) {
	f := setup(t, "One", "Two", "Three")

	n := len(f.GitHub.Requests())
	f.move(f.ada, 3, "backlog", 1).Expect(http.StatusOK, "")
	if got := f.since(n); len(got) != 0 {
		t.Fatalf("a reorder called GitHub: %v", got)
	}
	if got := f.order("backlog"); !slices.Equal(got, []int{3, 1, 2}) {
		t.Fatalf("after moving 3 before 1: %v", got)
	}
	if _, s := f.task(3); s.Position != 0 || s.Version != 2 {
		t.Fatalf("task 3 = %+v, want position 1 - 1 and version 2", s)
	}

	f.move(f.ada, 3, "backlog", 2).Expect(http.StatusOK, "")
	if _, s := f.task(3); s.Position != 1.5 {
		t.Fatalf("task 3 at %v, want midway between 1 and 2", s.Position)
	}
	f.move(f.ada, 1, "backlog", 0).Expect(http.StatusOK, "")
	if got := f.order("backlog"); !slices.Equal(got, []int{3, 2, 1}) {
		t.Fatalf("after moving 1 to the end: %v", got)
	}
	if _, s := f.task(1); s.Position != 3 {
		t.Fatalf("task 1 at %v, want after 2's 2", s.Position)
	}
}

func TestMoveRenumbersATinyGap(t *testing.T) {
	f := setup(t, "One", "Two", "Three")
	f.exec(`update tasks set position = 1 + 1e-10 where issue_number = 2`)

	f.move(f.ada, 3, "backlog", 2).Expect(http.StatusOK, "")
	if got := f.order("backlog"); !slices.Equal(got, []int{1, 3, 2}) {
		t.Fatalf("backlog = %v, want 3 between 1 and 2", got)
	}
	var positions []float64
	for _, number := range []int{1, 3, 2} {
		_, s := f.task(number)
		positions = append(positions, s.Position)
	}
	if !slices.Equal(positions, []float64{1, 1.5, 2}) {
		t.Fatalf("positions = %v, want the column renumbered 1, 2 and 3 placed between", positions)
	}
}

func TestMoveConflict(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	f.move(f.ada, 1, "testing", 0).Expect(http.StatusOK, "")

	stale := map[string]any{"columnId": f.column("closed"), "version": 1}
	res := f.Do("POST", f.path("/tasks/%d/move", id), stale, f.ada)
	res.Expect(http.StatusConflict, "conflict")
	var body struct{ Task task }
	res.JSON(&body)
	if body.Task.ID != id || body.Task.Version != 2 || body.Task.ColumnID != f.column("testing") {
		t.Fatalf("conflict returned task %+v, want the current one", body.Task)
	}
	if is := f.repo.Issue(1); is.State != "open" {
		t.Fatal("a conflicting move closed the issue")
	}
}

func TestMoveWhenGitHubFails(t *testing.T) {
	for _, c := range []struct {
		route  string
		status int
		want   int
		code   string
	}{
		{"POST /repos/{owner}/{name}/issues/{number}/labels", http.StatusInternalServerError, http.StatusBadGateway, "github_error"},
		{"DELETE /repos/{owner}/{name}/issues/{number}/labels/{label}", http.StatusForbidden, http.StatusForbidden, "forbidden"},
		{"PATCH /repos/{owner}/{name}/issues/{number}", http.StatusNotFound, http.StatusForbidden, "forbidden"},
	} {
		f := setup(t, "One")
		_, before := f.task(1)
		f.GitHub.Fail(c.route, c.status)
		f.move(f.ada, 1, "closed", 0).Expect(c.want, c.code)
		if _, after := f.task(1); after != before {
			t.Errorf("%s failing: task = %+v, want it unchanged from %+v", c.route, after, before)
		}
	}
}

func TestMoveNeedsWrite(t *testing.T) {
	f := setup(t, "One")
	f.move(f.reader(), 1, "testing", 0).Expect(http.StatusForbidden, "forbidden")
	f.move(f.stranger(), 1, "testing", 0).Expect(http.StatusNotFound, "not_found")
	f.move(nil, 1, "testing", 0).Expect(http.StatusUnauthorized, "unauthorized")
	if _, s := f.task(1); s.Column != "backlog" || s.Version != 1 {
		t.Fatalf("task = %+v, want it unmoved", s)
	}
}

func TestMoveValidates(t *testing.T) {
	f := setup(t, "One", "Two")
	other := f.GitHub.AddRepo("octo", "other")
	other.Grant(f.token, "push")
	f.AddProject(f.ada, "octo", "other")
	var foreign int64
	f.DB.QueryRow(t.Context(), `select id from columns where project_id <> $1 and key = 'testing'`, f.project).Scan(&foreign)
	one, _ := f.task(1)
	two, _ := f.task(2)
	move := func(id int64, body any) *apptest.Response {
		return f.Do("POST", f.path("/tasks/%d/move", id), body, f.ada)
	}
	backlog := f.column("backlog")

	move(999, map[string]any{"columnId": backlog, "version": 1}).Expect(http.StatusNotFound, "not_found")
	move(one, map[string]any{"columnId": foreign, "version": 1}).Expect(http.StatusBadRequest, "invalid_column")
	move(one, map[string]any{"version": 1}).Expect(http.StatusBadRequest, "invalid_column")
	move(one, map[string]any{"columnId": f.column("testing"), "beforeTaskId": two, "version": 1}).Expect(http.StatusBadRequest, "invalid_before_task")
	move(one, map[string]any{"columnId": backlog, "beforeTaskId": one, "version": 1}).Expect(http.StatusBadRequest, "invalid_before_task")
	move(one, map[string]any{"columnId": backlog, "beforeTaskId": 999, "version": 1}).Expect(http.StatusBadRequest, "invalid_before_task")
	move(one, `{`).Expect(http.StatusBadRequest, "bad_request")
	if _, s := f.task(1); s.Column != "backlog" || s.Version != 1 {
		t.Fatalf("task = %+v, want it unmoved", s)
	}
}
