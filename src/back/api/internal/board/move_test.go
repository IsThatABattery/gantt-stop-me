package board_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

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
	if _, labels := f.details(1); !slices.Equal(labels, []label{{"bug", "ededed"}, {"board:testing", "d4c5f9"}}) {
		t.Fatalf("stored labels = %v, want the issue's after the relabel", labels)
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
	if _, labels := f.details(1); !slices.Equal(labels, []label{{"board:closed", "0e8a16"}}) {
		t.Fatalf("stored labels after moving into Closed = %v", labels)
	}

	f.move(f.ada, 1, "user-check", 0).Expect(http.StatusOK, "")
	if is := f.repo.Issue(1); is.State != "open" || !slices.Equal(is.Labels, []string{"board:user-check"}) {
		t.Fatalf("issue after moving out of Closed = %+v", is)
	}
	if _, s := f.task(1); s.State != "open" || s.Column != "user-check" || s.Version != 3 {
		t.Fatalf("stored task = %+v", s)
	}
	if _, labels := f.details(1); !slices.Equal(labels, []label{{"board:user-check", "f9d0c4"}}) {
		t.Fatalf("stored labels after moving out of Closed = %v", labels)
	}
}

func TestMoveWithinColumn(t *testing.T) {
	f := setup(t, "One", "Two", "Three")
	f.exec(`update tasks set labels = '[{"name": "bug", "color": "ededed"}]' where issue_number = 3`)

	n := len(f.GitHub.Requests())
	f.move(f.ada, 3, "backlog", 1).Expect(http.StatusOK, "")
	if got := f.since(n); len(got) != 0 {
		t.Fatalf("a reorder called GitHub: %v", got)
	}
	if _, labels := f.details(3); !slices.Equal(labels, []label{{"bug", "ededed"}}) {
		t.Fatalf("stored labels after a reorder = %v, want them kept", labels)
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
		{"POST /repositories/{id}/issues/{number}/labels", http.StatusInternalServerError, http.StatusBadGateway, "github_error"},
		{"DELETE /repositories/{id}/issues/{number}/labels/{label}", http.StatusForbidden, http.StatusForbidden, "forbidden"},
		{"PATCH /repositories/{id}/issues/{number}", http.StatusNotFound, http.StatusForbidden, "forbidden"},
	} {
		f := setup(t, "One")
		f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Labels = []string{"board:backlog"} }) // for the move to remove
		_, before := f.task(1)
		f.GitHub.Fail(c.route, c.status)
		f.move(f.ada, 1, "closed", 0).Expect(c.want, c.code)
		if _, after := f.task(1); after != before {
			t.Errorf("%s failing: task = %+v, want it unchanged from %+v", c.route, after, before)
		}
		if _, labels := f.details(1); len(labels) != 0 {
			t.Errorf("%s failing: stored labels = %v, want them unchanged from none", c.route, labels)
		}
	}
}

func TestMoveUndoesLabelsWhenStateChangeFails(t *testing.T) {
	for _, c := range []struct{ from, to, label, state string }{
		{"testing", "closed", "board:testing", "open"},
		{"closed", "testing", "board:closed", "closed"},
	} {
		f := setup(t, "One")
		f.move(f.ada, 1, c.from, 0).Expect(http.StatusOK, "")
		f.GitHub.Fail("PATCH /repositories/{id}/issues/{number}", http.StatusBadGateway)
		f.move(f.ada, 1, c.to, 0).Expect(http.StatusBadGateway, "github_error")
		if is := f.repo.Issue(1); is.State != c.state || !slices.Equal(is.Labels, []string{c.label}) {
			t.Errorf("%s to %s with PATCH failing: issue = %s %v, want %s with only %s", c.from, c.to, is.State, is.Labels, c.state, c.label)
		}
	}
}

func TestMoveOutOfClosedDropsStaleLabels(t *testing.T) {
	f := setup(t, "One")
	f.move(f.ada, 1, "testing", 0).Expect(http.StatusOK, "")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.State = "closed" }) // closed on GitHub, say by a pull request: keeps board:testing
	f.dueSync()
	f.board()
	f.move(f.ada, 1, "backlog", 0).Expect(http.StatusOK, "")
	if is := f.repo.Issue(1); is.State != "open" || !slices.Equal(is.Labels, []string{"board:backlog"}) {
		t.Fatalf("issue = %s %v, want it reopened with only board:backlog", is.State, is.Labels)
	}
	_, moved := f.task(1)
	f.dueSync()
	f.board()
	if _, s := f.task(1); s != moved {
		t.Fatalf("the next sync changed the task from %+v to %+v, want the move kept", moved, s)
	}
}

// slowGitHub delays every request to GitHub.
type slowGitHub struct{ http.RoundTripper }

func (s slowGitHub) RoundTrip(r *http.Request) (*http.Response, error) {
	time.Sleep(300 * time.Millisecond)
	return s.RoundTripper.RoundTrip(r)
}

func TestQueuedMovesDoNotStarveThePool(t *testing.T) {
	f := setup(t)
	n := int(f.DB.Config().MaxConns) + 2 // more moves than the pool has connections
	for i := range n {
		f.repo.AddIssue(fmt.Sprint("Task ", i+1))
	}
	f.dueSync()
	f.board()
	col := f.column("testing")
	var ids []int64
	for i := range n {
		id, _ := f.task(i + 1)
		ids = append(ids, id)
	}
	f.Config.GitHub.HTTP = &http.Client{Transport: slowGitHub{f.Config.GitHub.HTTP.Transport}}

	codes := make(chan int, n)
	for _, id := range ids {
		go func() {
			body := strings.NewReader(fmt.Sprintf(`{"columnId": %d, "version": 1}`, col))
			req, _ := http.NewRequest("POST", f.Server.URL+f.path("/tasks/%d/move", id), body)
			req.Header.Set("Origin", apptest.PublicURL)
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(f.ada)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				codes <- 0
				return
			}
			res.Body.Close()
			codes <- res.StatusCode
		}()
	}
	time.Sleep(150 * time.Millisecond) // until the moves queue behind the first
	start := time.Now()
	f.Do("GET", "/api/me", nil, f.ada).Expect(http.StatusOK, "")
	if d := time.Since(start); d > 250*time.Millisecond {
		t.Errorf("GET /api/me took %v behind %d queued moves, want it served at once", d, n)
	}
	for range n {
		if code := <-codes; code != http.StatusOK {
			t.Errorf("a move answered %d, want 200", code)
		}
	}
	if got := f.order("testing"); len(got) != n {
		t.Fatalf("testing holds %d tasks, want all %d moved", len(got), n)
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
