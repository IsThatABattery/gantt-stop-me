package board_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"gantt-stop-me/api/internal/github/githubtest"
)

// listings counts the issue list requests among reqs.
func listings(reqs []string) int {
	n := 0
	for _, r := range reqs {
		if strings.HasPrefix(r, "GET /repos/octo/hello/issues?") {
			n++
		}
	}
	return n
}

// synced returns the project's synced_at and issues_etag.
func (f *fixture) synced() (time.Time, *string) {
	f.t.Helper()
	var at time.Time
	var etag *string
	if err := f.DB.QueryRow(context.Background(), `select synced_at, issues_etag from projects where id = $1`, f.project).Scan(&at, &etag); err != nil {
		f.t.Fatal(err)
	}
	return at, etag
}

func TestSyncAddsNewIssues(t *testing.T) {
	f := setup(t, "One")
	f.repo.AddIssue("Two")
	f.repo.AddPullRequest("A pull request")
	f.repo.AddIssue("Labeled", "board:testing")
	closed := f.repo.AddIssue("Closed on arrival")
	f.repo.EditIssue(closed, func(is *githubtest.Issue) { is.State = "closed" })
	f.dueSync()

	if got := f.order("backlog"); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("backlog = %v, want 1 then the new 2", got)
	}
	if got := f.order("testing"); !slices.Equal(got, []int{4}) {
		t.Fatalf("testing = %v, want the labeled 4", got)
	}
	if n := len(f.board().Tasks); n != 3 {
		t.Fatalf("%d tasks, want no task for the pull request or the closed issue", n)
	}
}

func TestSyncMovesClosedAndReopenedIssues(t *testing.T) {
	f := setup(t, "One", "Two")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.State = "closed"; is.Title = "One, done" })
	f.dueSync()
	f.board()
	if _, s := f.task(1); s.Column != "closed" || s.State != "closed" || s.Title != "One, done" || s.Version != 2 {
		t.Fatalf("closed on GitHub: task = %+v, want it in Closed, closed, retitled, version 2", s)
	}
	if _, s := f.task(2); s.Column != "backlog" || s.Version != 1 {
		t.Fatalf("untouched task = %+v", s)
	}

	// Reopened, it leaves Closed for Backlog even if it still carries board:closed.
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.State = "open"; is.Labels = []string{"board:closed"} })
	f.dueSync()
	f.board()
	if _, s := f.task(1); s.Column != "backlog" || s.State != "open" || s.Version != 3 {
		t.Fatalf("reopened on GitHub: task = %+v, want it in Backlog, open, version 3", s)
	}
	if got := f.order("backlog"); !slices.Equal(got, []int{2, 1}) {
		t.Fatalf("backlog = %v, want the reopened 1 at the end", got)
	}
}

func TestSyncFollowsLabelChanges(t *testing.T) {
	f := setup(t, "One", "Two", "Three")
	f.exec(`update tasks set column_id = $1 where issue_number = 3`, f.column("testing"))
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Labels = []string{"board:testing"} })
	f.repo.EditIssue(2, func(is *githubtest.Issue) { is.Labels = []string{"board:backlog", "board:user-check"} })
	f.repo.EditIssue(3, func(is *githubtest.Issue) { is.Labels = []string{"board:testing"} })
	f.dueSync()
	f.board()

	if _, s := f.task(1); s.Column != "testing" || s.Version != 2 {
		t.Fatalf("relabeled testing: task = %+v", s)
	}
	if _, s := f.task(2); s.Column != "user-check" {
		t.Fatalf("labeled with its own column and user-check: task = %+v, want User Check", s)
	}
	if _, s := f.task(3); s.Column != "testing" || s.Version != 1 {
		t.Fatalf("labeled with its own column: task = %+v, want it left alone", s)
	}
}

func TestSyncAtMostEvery30Seconds(t *testing.T) {
	f := setup(t, "One")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Title = "Renamed" })

	n := len(f.GitHub.Requests())
	f.board() // the add just synced
	f.exec(`update projects set synced_at = now() - interval '29 seconds'`)
	f.board()
	if _, s := f.task(1); s.Title != "One" || listings(f.since(n)) != 0 {
		t.Fatalf("synced within 30 seconds: task %+v, requests %v", s, f.since(n))
	}
	f.exec(`update projects set synced_at = now() - interval '31 seconds'`)
	f.board()
	f.board()
	if _, s := f.task(1); s.Title != "Renamed" || listings(f.since(n)) != 1 {
		t.Fatalf("after 30 seconds: task %+v, requests %v; want one sync", s, f.since(n))
	}
}

func TestSyncUsesETag(t *testing.T) {
	f := setup(t, "One")
	f.dueSync()
	f.board()
	if _, etag := f.synced(); etag == nil {
		t.Fatal("sync stored no ETag")
	}

	// GitHub answers 304, so the sync keeps the board as it is, even this tampered title.
	f.exec(`update tasks set title = 'Tampered'`)
	f.dueSync()
	n := len(f.GitHub.Requests())
	f.board()
	if _, s := f.task(1); s.Title != "Tampered" || listings(f.since(n)) != 1 {
		t.Fatalf("unchanged issues: task %+v, requests %v; want one listing answered 304", s, f.since(n))
	}

	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Title = "Renamed" })
	f.dueSync()
	f.board()
	if _, s := f.task(1); s.Title != "Renamed" {
		t.Fatalf("changed issues: task %+v", s)
	}
}

func TestSyncFailureServesCachedBoard(t *testing.T) {
	f := setup(t, "One")
	f.repo.AddIssue("Two")
	f.GitHub.Fail("GET /repos/{owner}/{name}/issues", http.StatusInternalServerError)
	f.dueSync()
	before, _ := f.synced()

	if n := len(f.board().Tasks); n != 1 {
		t.Fatalf("board during a failing sync has %d tasks, want the cached 1", n)
	}
	if after, _ := f.synced(); !after.Equal(before) {
		t.Fatalf("synced_at = %v after a failed sync, want it put back to %v", after, before)
	}

	f.GitHub.Fail("GET /repos/{owner}/{name}/issues", 0)
	if n := len(f.board().Tasks); n != 2 {
		t.Fatalf("the next load has %d tasks, want it to retry the sync and find 2", n)
	}
}

func TestSyncStopsAtTheLastSync(t *testing.T) {
	f := setup(t, "A", "B", "C")
	f.GitHub.PageSize = 1
	f.dueSync()
	old := time.Now().Add(-time.Hour)
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Title = "A2"; is.UpdatedAt = old })
	f.repo.EditIssue(2, func(is *githubtest.Issue) { is.Title = "B2"; is.UpdatedAt = old })
	f.repo.EditIssue(3, func(is *githubtest.Issue) { is.Title = "C2" })

	n := len(f.GitHub.Requests())
	f.board()
	// Newest first: C, then A, which is older than the last sync, so B on page 3 is never read.
	if got := listings(f.since(n)); got != 2 {
		t.Fatalf("sync read %d pages, want 2: %v", got, f.since(n))
	}
	var titles []string
	for _, number := range []int{1, 2, 3} {
		_, s := f.task(number)
		titles = append(titles, s.Title)
	}
	if !slices.Equal(titles, []string{"A2", "B", "C2"}) {
		t.Fatalf("titles = %v", titles)
	}
}

func TestSyncSkipsTasksMovedSinceItBegan(t *testing.T) {
	f := setup(t, "One")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Title = "Renamed"; is.Labels = []string{"board:testing"} })
	f.exec(`update tasks set updated_at = now() + interval '1 hour'`) // as if moved after the sync's claim
	f.dueSync()
	f.board()
	if _, s := f.task(1); s.Title != "One" || s.Column != "backlog" {
		t.Fatalf("task moved during the sync = %+v, want it untouched", s)
	}
	if _, etag := f.synced(); etag != nil {
		t.Fatal("a sync that skipped a task stored its ETag, so the next would not reconcile it")
	}

	f.exec(`update tasks set updated_at = now() - interval '1 hour'`)
	f.dueSync()
	f.board()
	if _, s := f.task(1); s.Title != "Renamed" || s.Column != "testing" {
		t.Fatalf("next sync: task = %+v, want it reconciled", s)
	}
}
