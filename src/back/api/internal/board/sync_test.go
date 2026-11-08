package board_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/store"
	"gantt-stop-me/api/migrations"
)

// listings counts the issue list requests among reqs, for octo/hello, which as the fake's first repository has id 1.
func listings(reqs []string) int {
	n := 0
	for _, r := range reqs {
		if strings.HasPrefix(r, "GET /repositories/1/issues?") {
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

func TestSyncStoresBodyAndLabels(t *testing.T) {
	f := setup(t, "One")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "Changed."; is.Labels = []string{"bug"} })
	f.repo.AddIssue("Two", "board:testing")
	f.repo.EditIssue(2, func(is *githubtest.Issue) { is.Body = "New." })
	f.dueSync()
	f.board()
	if body, labels := f.details(1); body != "Changed." || !slices.Equal(labels, []label{{"bug", "ededed"}}) {
		t.Fatalf("changed issue: body %q, labels %v", body, labels)
	}
	if body, labels := f.details(2); body != "New." || !slices.Equal(labels, []label{{"board:testing", "d4c5f9"}}) {
		t.Fatalf("new issue: body %q, labels %v", body, labels)
	}

	// GitHub sends a cleared body as null.
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = ""; is.Labels = nil })
	f.dueSync()
	f.board()
	if body, labels := f.details(1); body != "" || len(labels) != 0 {
		t.Fatalf("cleared issue: body %q, labels %v", body, labels)
	}
}

// detailedIssues gives issues 1 to n the body "Body <number>" and the label bug, oldest first, all changed an hour
// ago, before any recent sync's cutoff.
func (f *fixture) detailedIssues(n int) {
	old := time.Now().Add(-time.Hour)
	for number := 1; number <= n; number++ {
		f.repo.EditIssue(number, func(is *githubtest.Issue) {
			is.Body, is.Labels, is.UpdatedAt = fmt.Sprint("Body ", number), []string{"bug"}, old.Add(time.Duration(number)*time.Minute)
		})
	}
}

// backfilled fails the test unless GET task answers issues 1 to n with what detailedIssues gave them.
func (f *fixture) backfilled(n int) {
	f.t.Helper()
	for number := 1; number <= n; number++ {
		if body, labels := f.details(number); body != fmt.Sprint("Body ", number) || !slices.Equal(labels, []label{{"bug", "ededed"}}) {
			f.t.Errorf("issue %d: body %q, labels %v; want them backfilled", number, body, labels)
		}
	}
}

// upgrade undoes migration 005, then applies it again as the upgraded API does at startup, a minute after the last sync.
func (f *fixture) upgrade() {
	f.t.Helper()
	f.exec(`alter table tasks drop column body, drop column labels`)
	f.exec(`alter table projects drop column backfill_pending`)
	f.exec(`delete from schema_migrations where version = '005_task_details.sql'`)
	f.exec(`update projects set synced_at = now() - interval '1 minute'`)
	if err := store.Migrate(context.Background(), f.DB, migrations.FS); err != nil {
		f.t.Fatal(err)
	}
}

// A project synced before migration 005 has tasks with no body or labels, and an ETag and a cutoff its next sync
// would stop at. 005 makes that sync read every issue.
func TestSyncBackfillsTasksFromBeforeTaskDetails(t *testing.T) {
	f := setup(t, "A", "B", "C")
	f.GitHub.PageSize = 1
	f.detailedIssues(3)
	f.dueSync()
	f.board() // the last sync before the upgrade, which stores the ETag
	f.upgrade()
	f.board()
	f.backfilled(3)
}

// A sync that claims while the backfill's first read is still paging reads every issue too, so the backfill
// is not lost when that first read then fails.
func TestSyncBackfillSurvivesAnOverlappingSync(t *testing.T) {
	f := setup(t, "A", "B", "C", "D", "E")
	f.GitHub.PageSize = 1
	f.detailedIssues(5)
	f.dueSync()
	f.board()
	f.upgrade()

	stalled, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.intercept(func(r *http.Request, next http.RoundTripper) (*http.Response, error) {
		first := false
		if r.URL.Query().Get("page") == "2" {
			once.Do(func() { first = true })
		}
		if first { // the first read stalls on its second page, then fails
			close(stalled)
			<-release
			return nil, errors.New("connection reset")
		}
		return next.RoundTrip(r)
	})
	done := make(chan int)
	go func() { done <- f.Do("GET", f.path("/board"), nil, f.ada).Status }()
	<-stalled
	f.dueSync()  // 30 seconds later,
	f.details(5) // a task's detail view syncs again
	close(release)
	if status := <-done; status != http.StatusOK {
		t.Fatalf("board during the failed read answered %d", status)
	}

	f.dueSync()
	f.board()
	f.backfilled(5)
}

// The backfill's read finishes and is saved even when the client, or a proxy, gives up on the request it runs in.
func TestSyncBackfillOutlivesItsRequest(t *testing.T) {
	f := setup(t, "A", "B", "C", "D", "E")
	f.GitHub.PageSize = 1
	f.detailedIssues(5)
	f.dueSync()
	f.board()
	f.upgrade()
	f.intercept(func(r *http.Request, next http.RoundTripper) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/issues") {
			select {
			case <-time.After(200 * time.Millisecond): // so the 5 pages take a second
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		}
		return next.RoundTrip(r)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", f.Server.URL+f.path("/board"), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", apptest.PublicURL)
	req.AddCookie(f.ada)
	if res, err := http.DefaultClient.Do(req); err == nil {
		res.Body.Close()
		t.Fatalf("board answered %d within 500ms, before the read could finish", res.StatusCode)
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		var filled int
		var pending bool
		err := f.DB.QueryRow(context.Background(), `
			select (select count(*) from tasks where body <> ''), backfill_pending from projects where id = $1`, f.project).Scan(&filled, &pending)
		if err != nil {
			t.Fatal(err)
		}
		if filled == 5 && !pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of 5 tasks backfilled, backfill pending: %v", filled, pending)
		}
	}
	f.backfilled(5)
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
	for _, list := range []string{"issues", "labels"} {
		t.Run(list, func(t *testing.T) {
			f := setup(t, "One")
			f.repo.AddIssue("Two")
			f.GitHub.Fail("GET /repositories/{id}/"+list, http.StatusInternalServerError)
			f.dueSync()
			before, _ := f.synced()

			if n := len(f.board().Tasks); n != 1 {
				t.Fatalf("board during a failing sync has %d tasks, want the cached 1", n)
			}
			if after, _ := f.synced(); !after.Equal(before) {
				t.Fatalf("synced_at = %v after a failed sync, want it put back to %v", after, before)
			}

			f.GitHub.Fail("GET /repositories/{id}/"+list, 0)
			if n := len(f.board().Tasks); n != 2 {
				t.Fatalf("the next load has %d tasks, want it to retry the sync and find 2", n)
			}
		})
	}
}

// labeledTask gives issue 1 these labels and syncs them, then dates every change to before the next sync's cutoff,
// issue 2's last, so later syncs stop at issue 2: only the label list can bring issue 1 a label edit.
func (f *fixture) labeledTask(labels ...string) {
	f.t.Helper()
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Labels = labels })
	f.dueSync()
	f.board()
	f.GitHub.PageSize = 1
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.UpdatedAt = time.Now().Add(-time.Hour) })
	f.repo.EditIssue(2, func(is *githubtest.Issue) { is.UpdatedAt = time.Now().Add(-30 * time.Minute) })
}

// stamp returns the version and updated_at of issue number's task.
func (f *fixture) stamp(number int) (int, time.Time) {
	f.t.Helper()
	var version int
	var at time.Time
	err := f.DB.QueryRow(context.Background(), `select version, updated_at from tasks where project_id = $1 and issue_number = $2`,
		f.project, number).Scan(&version, &at)
	if err != nil {
		f.t.Fatal(err)
	}
	return version, at
}

func TestSyncFollowsRepositoryLabelEdits(t *testing.T) {
	f := setup(t, "One", "Two")
	f.labeledTask("bug", "board:backlog", "wontfix")
	f.exec(`update tasks set labels = labels || '[{"name": "legacy", "color": "000000"}]' where issue_number = 1`) // no id, as before 006
	version, at := f.stamp(1)
	for _, c := range []struct {
		edit string
		do   func()
		want []label
	}{
		{"renamed", func() { f.repo.EditLabel("bug", func(l *githubtest.Label) { l.Name = "defect" }) },
			[]label{{"defect", "ededed"}, {"board:backlog", "c5def5"}, {"wontfix", "ededed"}, {"legacy", "000000"}}},
		{"recolored", func() { f.repo.EditLabel("wontfix", func(l *githubtest.Label) { l.Color = "ffffff" }) },
			[]label{{"defect", "ededed"}, {"board:backlog", "c5def5"}, {"wontfix", "ffffff"}, {"legacy", "000000"}}},
		{"deleted", func() { f.repo.DeleteLabel("defect") },
			[]label{{"board:backlog", "c5def5"}, {"wontfix", "ffffff"}, {"legacy", "000000"}}},
	} {
		c.do()
		f.dueSync()
		if _, labels := f.details(1); !slices.Equal(labels, c.want) {
			t.Fatalf("label %s on GitHub: GET task labels %v, want %v", c.edit, labels, c.want)
		}
	}
	if v, a := f.stamp(1); v != version || !a.Equal(at) {
		t.Fatalf("version %d, updated_at %v after the label edits; want %d, %v untouched", v, a, version, at)
	}
}

// Deleting every label a task carries, down to the repository's last, leaves it with none, and syncs keep going.
func TestSyncDeletesEveryLabelOfATask(t *testing.T) {
	f := setup(t, "One", "Two")
	f.labeledTask("bug", "board:backlog")
	for _, name := range f.repo.Labels() {
		f.repo.DeleteLabel(name)
	}
	f.dueSync()
	if _, labels := f.details(1); len(labels) != 0 {
		t.Fatalf("every label deleted on GitHub: GET task labels %v, want none", labels)
	}
	f.repo.AddIssue("Three")
	f.dueSync()
	if n := len(f.board().Tasks); n != 3 {
		t.Fatalf("%d tasks, want the new issue imported", n)
	}
}

// A sync whose issue read began before a label rename must not leave the old name once a newer sync has stored
// the new list's ETag.
func TestSyncLabelRenameSurvivesAnOverlappingSync(t *testing.T) {
	f := setup(t, "One", "Two", "Three")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Labels = []string{"bug"} })
	f.dueSync()
	f.board()
	f.GitHub.PageSize = 1
	now := time.Now()
	for n, ago := range map[int]time.Duration{1: 30, 2: 20, 3: 10} {
		f.repo.EditIssue(n, func(is *githubtest.Issue) { is.UpdatedAt = now.Add(-ago * time.Minute) })
	}
	f.exec(`update projects set synced_at = now() - interval '1 hour' where id = $1`, f.project)
	stalled, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.intercept(func(r *http.Request, next http.RoundTripper) (*http.Response, error) {
		res, err := next.RoundTrip(r)
		first := false
		if strings.HasSuffix(r.URL.Path, "/issues") && r.URL.Query().Get("page") == "3" {
			once.Do(func() { first = true })
		}
		if first && err == nil { // the first read holds #1 with bug while the label is renamed and another sync runs
			body, _ := io.ReadAll(res.Body)
			res.Body.Close()
			res.Body = io.NopCloser(bytes.NewReader(body))
			close(stalled)
			<-release
		}
		return res, err
	})
	done := make(chan int)
	go func() { done <- f.Do("GET", f.path("/board"), nil, f.ada).Status }()
	<-stalled
	f.repo.EditLabel("bug", func(l *githubtest.Label) { l.Name = "defect" })
	f.dueSync()
	f.board()
	close(release)
	if s := <-done; s != http.StatusOK {
		t.Fatalf("first sync's board answered %d", s)
	}
	f.dueSync()
	if _, labels := f.details(1); !slices.Equal(labels, []label{{"defect", "ededed"}}) {
		t.Fatalf("labels %v after overlapping syncs, want the rename", labels)
	}
}

// A label deleted between two page reads shifts the next one onto the page already read; that label must survive.
func TestSyncLabelsChangedBetweenPages(t *testing.T) {
	f := setup(t, "One", "Two")
	var names []string
	for i := range 95 { // after the 5 board labels: a full first page, then triage alone on the second
		names = append(names, fmt.Sprint("l", i))
	}
	f.repo.EditIssue(2, func(is *githubtest.Issue) { is.Labels = names })
	f.labeledTask("triage")
	var once sync.Once
	f.intercept(func(r *http.Request, next http.RoundTripper) (*http.Response, error) {
		res, err := next.RoundTrip(r)
		if strings.HasSuffix(r.URL.Path, "/labels") {
			once.Do(func() { f.repo.DeleteLabel("l0") }) // right after the first page
		}
		return res, err
	})
	for range 3 {
		f.dueSync()
		f.board()
	}
	if _, labels := f.details(1); !slices.Equal(labels, []label{{"triage", "ededed"}}) {
		t.Fatalf("labels %v after a deletion mid-read, want triage kept", labels)
	}
	if _, labels := f.details(2); len(labels) != 94 || slices.Contains(labels, label{"l0", "ededed"}) {
		t.Fatalf("%d labels on issue 2, want the 94 left after deleting l0", len(labels))
	}
}

// labelsETag returns the project's labels_etag.
func (f *fixture) labelsETag() *string {
	f.t.Helper()
	var etag *string
	if err := f.DB.QueryRow(context.Background(), `select labels_etag from projects where id = $1`, f.project).Scan(&etag); err != nil {
		f.t.Fatal(err)
	}
	return etag
}

func TestSyncUsesLabelsETag(t *testing.T) {
	f := setup(t, "One", "Two")
	f.labeledTask("bug")
	etag := f.labelsETag()
	if etag == nil {
		t.Fatal("sync stored no labels ETag")
	}

	// GitHub answers 304, so the sync keeps the stored labels as they are, even this tampered name, and still counts.
	f.exec(`update tasks set labels = jsonb_set(labels, '{0,name}', '"tampered"') where issue_number = 1`)
	version, at := f.stamp(1)
	f.dueSync()
	n := len(f.GitHub.Requests())
	if _, labels := f.details(1); !slices.Equal(labels, []label{{"tampered", "ededed"}}) {
		t.Fatalf("unchanged label list: labels %v, want the stored ones", labels)
	}
	if got := f.since(n); !slices.Contains(got, "GET /repositories/1/labels?per_page=100") {
		t.Fatalf("requests %v, want the label list", got)
	}
	if v, a := f.stamp(1); v != version || !a.Equal(at) {
		t.Fatalf("version %d, updated_at %v; want %d, %v untouched", v, a, version, at)
	}
	claimed, _ := f.synced()
	if kept := f.labelsETag(); kept == nil || *kept != *etag || time.Since(claimed) > 30*time.Second {
		t.Fatalf("labels ETag %v, synced at %v; want the ETag kept and the sync's claim standing", kept, claimed)
	}
}

// An ETag covers only the first page, so a label list that fills one is read in full on every sync.
func TestSyncRereadsLabelsOverOnePage(t *testing.T) {
	f := setup(t, "One", "Two")
	var names []string
	for i := range 96 { // after the 5 board labels: 101, one past a page
		names = append(names, fmt.Sprint("l", i))
	}
	f.labeledTask(names...)
	if etag := f.labelsETag(); etag != nil {
		t.Fatalf("labels ETag %q stored for two pages of labels", *etag)
	}

	f.repo.EditLabel("l95", func(l *githubtest.Label) { l.Name = "last" }) // on the second page
	f.dueSync()
	if _, labels := f.details(1); len(labels) != 96 || labels[95] != (label{"last", "ededed"}) {
		t.Fatalf("%d labels, want 96 ending in the one renamed on the second page", len(labels))
	}
	if etag := f.labelsETag(); etag != nil {
		t.Fatalf("labels ETag %q stored for two pages of labels", *etag)
	}

	f.repo.DeleteLabel("l0") // 100 left: one full page, which a label added later would leave unchanged
	f.dueSync()
	f.board()
	if etag := f.labelsETag(); etag != nil {
		t.Fatalf("labels ETag %q stored for a full page of labels", *etag)
	}
	f.repo.DeleteLabel("l1")
	f.dueSync()
	f.board()
	if etag := f.labelsETag(); etag == nil {
		t.Fatal("no labels ETag stored for 99 labels")
	}
}

func TestSyncLabelsWithRevokedCredential(t *testing.T) {
	f := setup(t, "One")
	f.dueSync()
	f.GitHub.Fail("GET /repositories/{id}/labels", http.StatusUnauthorized) // the token expired or was revoked
	f.Do("GET", f.path("/board"), nil, f.ada).Expect(http.StatusForbidden, "github_not_linked")
}

// Labels stored before migration 006 have no id. 006 makes the next sync read every issue, which stores them with ids,
// so a later label edit reaches them.
func TestSyncBackfillsLabelIDs(t *testing.T) {
	f := setup(t, "One", "Two")
	f.labeledTask("bug")
	f.exec(`update tasks set labels = '[{"name": "bug", "color": "ededed"}]' where issue_number = 1`)
	f.exec(`alter table projects drop column labels_etag`)
	f.exec(`delete from schema_migrations where version = '006_label_sync.sql'`)
	f.exec(`insert into projects (github_repo_id, owner, name) values (999, 'octo', 'uninitialized')`)
	if err := store.Migrate(context.Background(), f.DB, migrations.FS); err != nil {
		t.Fatal(err)
	}
	var initialized, uninitialized bool
	f.DB.QueryRow(t.Context(), `select backfill_pending from projects where id = $1`, f.project).Scan(&initialized)
	f.DB.QueryRow(t.Context(), `select backfill_pending from projects where synced_at is null`).Scan(&uninitialized)
	if !initialized || uninitialized {
		t.Fatalf("backfill pending: initialized project %v, uninitialized %v; want true, false", initialized, uninitialized)
	}

	f.dueSync()
	f.board()
	f.repo.EditLabel("bug", func(l *githubtest.Label) { l.Name = "defect" })
	f.dueSync()
	if _, labels := f.details(1); !slices.Equal(labels, []label{{"defect", "ededed"}}) {
		t.Fatalf("labels %v after the backfill and a rename, want the rename", labels)
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

func TestSyncReconcilesSkippedTaskOlderThanTheNextCutoff(t *testing.T) {
	f := setup(t, "A", "B", "C")
	f.GitHub.PageSize = 1 // stands in for 100+ issues updated since A's change
	now := time.Now()
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.State = "closed"; is.UpdatedAt = now.Add(-170 * time.Second) })
	f.repo.EditIssue(2, func(is *githubtest.Issue) { is.Title = "B2"; is.UpdatedAt = now.Add(-160 * time.Second) })
	f.repo.EditIssue(3, func(is *githubtest.Issue) { is.Title = "C2" })
	f.exec(`update tasks set updated_at = now() + interval '1 hour' where issue_number = 1`) // reordered after the claim, no GitHub write
	f.dueSync()
	f.board()
	if _, s := f.task(1); s.State != "open" {
		t.Fatalf("first sync should skip A: %+v", s)
	}

	f.exec(`update tasks set updated_at = now() - interval '1 hour' where issue_number = 1`)
	f.exec(`update projects set synced_at = synced_at - interval '31 seconds'`) // the next load, 31 seconds later
	f.board()
	if _, s := f.task(1); s.Column != "closed" || s.State != "closed" {
		t.Fatalf("skipped task never reconciled: %+v, GitHub has it closed", s)
	}
}

func TestSyncSettlesOnTwoBoardLabels(t *testing.T) {
	f := setup(t, "One", "Two")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Labels = []string{"board:backlog", "board:in-progress"} })
	f.dueSync()
	f.board()
	_, first := f.task(1)
	if first.Column != "in-progress" {
		t.Fatalf("task = %+v, want it in In Progress, the label added on GitHub", first)
	}
	for i := range 3 {
		f.repo.EditIssue(2, func(is *githubtest.Issue) { is.Title += "!" }) // so the listing misses its ETag
		f.dueSync()
		f.board()
		if _, s := f.task(1); s != first {
			t.Fatalf("sync %d moved the task from %+v to %+v, though its labels did not change", i, first, s)
		}
	}
}

func TestSyncReopenIgnoresStaleLabel(t *testing.T) {
	f := setup(t, "One")
	f.move(f.ada, 1, "in-progress", 0).Expect(http.StatusOK, "") // the issue now carries board:in-progress
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.State = "closed" })
	f.dueSync()
	f.board()
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.State = "open" }) // labels untouched: still board:in-progress
	f.dueSync()
	f.board()
	if _, s := f.task(1); s.Column != "backlog" || s.State != "open" || s.Version != 4 {
		t.Fatalf("reopened on GitHub with its old label: task = %+v, want Backlog, open, version 4", s)
	}

	// It stays there when the issue is listed again with the same old label.
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Title = "One, again" })
	f.dueSync()
	f.board()
	if _, s := f.task(1); s.Column != "backlog" || s.Title != "One, again" || s.Version != 4 {
		t.Fatalf("next sync: task = %+v, want it left in Backlog at version 4", s)
	}
}

func TestSyncFollowsTheRepositoryAcrossRenames(t *testing.T) {
	f := setup(t, "One")
	bob := f.reader()
	f.board()
	f.repo.Rename("octo", "hello-old")
	other := f.GitHub.AddRepo("octo", "hello") // takes the old name; only ada sees it
	other.Grant(f.token, "push")
	other.AddIssue("Other #1")
	other.AddIssue("SECRET")
	f.repo.AddIssue("Two")
	f.dueSync()
	f.board()

	res := f.Do("GET", f.path("/board"), nil, bob)
	res.Expect(http.StatusOK, "")
	if strings.Contains(string(res.Body), "SECRET") {
		t.Fatal("the board shows an issue of the repository that took the old name")
	}
	if _, s := f.task(2); s.Title != "Two" {
		t.Fatalf("task 2 = %+v, want the renamed repository's issue", s)
	}
	f.move(f.ada, 1, "testing", 0).Expect(http.StatusOK, "")
	if len(other.Issue(1).Labels) != 0 || !slices.Contains(f.repo.Issue(1).Labels, "board:testing") {
		t.Fatal("the move labeled the other repository's issue")
	}

	// Once the access check asks GitHub again, the project shows the new name.
	f.exec(`delete from project_members`)
	if b := f.board(); b.Project.Name != "hello-old" {
		t.Fatalf("project name = %q, want hello-old", b.Project.Name)
	}
}
