package board_test

import (
	"maps"
	"net/http"
	"slices"
	"testing"
)

func TestShowBoard(t *testing.T) {
	f := setup(t, "One", "Two", "Three")
	f.exec(`update tasks set column_id = $1 where issue_number = 2`, f.column("testing"))
	f.exec(`update tasks set position = 10 where issue_number = 1`) // now below Three in Backlog

	b := f.board()
	if b.Project.ID != f.project || b.Project.Owner != "octo" || b.Project.Name != "hello" || b.Project.Permission != "write" {
		t.Fatalf("project = %+v", b.Project)
	}
	var keys []string
	for i, c := range b.Columns {
		keys = append(keys, c.Key)
		if c.Position != i || c.ClosesIssue != (c.Key == "closed") || c.ID != f.column(c.Key) {
			t.Errorf("column %+v", c)
		}
	}
	if !slices.Equal(keys, []string{"backlog", "in-progress", "testing", "user-check", "closed"}) || b.Columns[1].Name != "In Progress" {
		t.Fatalf("columns = %+v", b.Columns)
	}

	var numbers []int
	for _, task := range b.Tasks {
		numbers = append(numbers, task.IssueNumber)
	}
	if !slices.Equal(numbers, []int{3, 1, 2}) {
		t.Fatalf("tasks in order %v, want Backlog's 3 and 1, then Testing's 2", numbers)
	}
	one := b.Tasks[1]
	if one.Title != "One" || one.State != "open" || one.Version != 1 || one.Position != 10 || one.ColumnID != f.column("backlog") ||
		one.URL != f.GitHub.Server.URL+"/octo/hello/issues/1" {
		t.Fatalf("task = %+v", one)
	}
}

func TestShowBoardRunStatus(t *testing.T) {
	f := setup(t, "One", "Two")
	f.runs(1, "The build failed. The execution's log says why.", "cancelled", "error")
	b := f.board()
	got := map[int]string{}
	for _, task := range b.Tasks {
		got[task.IssueNumber] = orNull(task.RunStatus) + ", " + orNull(task.NeedsAttention)
	}
	if want := map[int]string{1: "error, The build failed. The execution's log says why.", 2: "null, null"}; !maps.Equal(got, want) {
		t.Fatalf("latest run status and needs attention by issue = %v, want %v", got, want)
	}
}

func TestShowBoardForReaders(t *testing.T) {
	f := setup(t, "One")
	res := f.Do("GET", f.path("/board"), nil, f.reader())
	res.Expect(http.StatusOK, "")
	var b board
	res.JSON(&b)
	if b.Project.Permission != "read" || len(b.Tasks) != 1 {
		t.Fatalf("reader got %+v", b)
	}
}

func TestShowBoardNeedsAccess(t *testing.T) {
	f := setup(t)
	f.Do("GET", f.path("/board"), nil, f.stranger()).Expect(http.StatusNotFound, "not_found")
}

func TestShowBoardNeedsSession(t *testing.T) {
	f := setup(t)
	f.Do("GET", f.path("/board"), nil).Expect(http.StatusUnauthorized, "unauthorized")
}

func TestShowBoardWithRevokedCredential(t *testing.T) {
	f := setup(t, "One")
	f.exec(`update projects set synced_at = now() - interval '31 seconds'`)
	f.GitHub.Fail("GET /repositories/{id}/issues", http.StatusUnauthorized) // the token expired or was revoked
	f.Do("GET", f.path("/board"), nil, f.ada).Expect(http.StatusForbidden, "github_not_linked")
}
