package automation_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/automation"
	"gantt-stop-me/api/internal/github/githubtest"
)

const (
	ambiguous   = "More than one open, ready pull request is linked to this issue."
	forkRefusal = "Not tested: this pull request comes from a fork. Automation tests only pull requests from branches of this repository."
)

// automated is a fixture with automation on, ada its owner, and a webhook secret.
func automated(t *testing.T) *fixture {
	f := on(t)
	f.secret = f.webhookSecret(f.ada)
	return f
}

// closing refers to these issues of the fixture's repository.
func (f *fixture) closing(issues ...int) []githubtest.Ref {
	var refs []githubtest.Ref
	for _, n := range issues {
		refs = append(refs, githubtest.Ref{RepoID: f.repo.ID, Number: n})
	}
	return refs
}

// pull opens a pull request by author from a branch of the repository, with head "a1", closing these issues of it,
// and returns its number.
func (f *fixture) pull(author string, draft bool, issues ...int) int {
	return f.repo.AddPull("Fix", githubtest.Pull{Author: author, HeadSHA: "a1", HeadRepo: f.repo.ID, Draft: draft, Closes: f.closing(issues...)})
}

// poll runs one poll of the project, failing the test on error.
func (f *fixture) poll() {
	f.t.Helper()
	if err := f.Automation.Poll(context.Background(), f.project); err != nil {
		f.t.Fatalf("poll: %v", err)
	}
}

// sign is the X-Hub-Signature-256 GitHub sends with body for a webhook with secret.
func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// deliver sends the project's webhook a delivery of event with body and signature, "" for none, as GitHub does: with
// no session or Origin. It waits for the work the delivery started, and returns the answer's status and error code.
func (f *fixture) deliver(signature, event, body string) (int, string) {
	f.t.Helper()
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/github/webhook/%d", f.Server.URL, f.project), strings.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", event)
	if signature != "" {
		req.Header.Set("X-Hub-Signature-256", signature)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error struct{ Code string }
	}
	data, _ := io.ReadAll(res.Body)
	json.Unmarshal(data, &out)
	f.Automation.Wait()
	return res.StatusCode, out.Error.Code
}

// hint delivers the pull_request event GitHub sends on a change to pull request number, and waits for it to be applied.
func (f *fixture) hint(number int) {
	f.t.Helper()
	body := fmt.Sprintf(`{"action": "synchronize", "number": %d, "pull_request": {"number": %d}}`, number, number)
	if status, _ := f.deliver(sign(f.secret, body), "pull_request", body); status != http.StatusAccepted {
		f.t.Fatalf("delivery answered %d, want 202", status)
	}
}

// paths are the ways a pull request's change reaches the API: a poll, a webhook delivery, or both in either order.
var paths = []struct {
	name string
	via  func(f *fixture, number int)
}{
	{"poll", func(f *fixture, n int) { f.poll() }},
	{"webhook", func(f *fixture, n int) { f.hint(n) }},
	{"poll then webhook", func(f *fixture, n int) { f.poll(); f.hint(n) }},
	{"webhook then poll", func(f *fixture, n int) { f.hint(n); f.poll() }},
}

// testRun is a stored test run, with 0 and "" for nulls.
type testRun struct {
	Issue    int
	PR       int
	Head     string
	Status   string
	Reason   string
	Finished bool
}

// runs returns the project's test runs, oldest first.
func (f *fixture) runs() []testRun {
	f.t.Helper()
	rows, _ := f.DB.Query(context.Background(), `
		select t.issue_number, coalesce(r.pr_number, 0), coalesce(r.head_sha, ''), r.status, coalesce(r.reason, ''),
			r.finished_at is not null
		from test_runs r join tasks t on t.id = r.task_id where t.project_id = $1 order by r.id`, f.project)
	runs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[testRun])
	if err != nil {
		f.t.Fatal(err)
	}
	return runs
}

// expectRuns fails the test unless the project's test runs are want.
func (f *fixture) expectRuns(want ...testRun) {
	f.t.Helper()
	if got := f.runs(); !slices.Equal(got, want) {
		f.t.Fatalf("test runs = %+v, want %+v", got, want)
	}
}

// taskState is a stored task, with "" for no needs_attention.
type taskState struct {
	Column         string // key
	Position       float64
	Version        int
	NeedsAttention string
}

// task returns the stored task of issue number.
func (f *fixture) task(number int) taskState {
	f.t.Helper()
	var s taskState
	err := f.DB.QueryRow(context.Background(), `
		select c.key, t.position, t.version, coalesce(t.needs_attention, '') from tasks t join columns c on c.id = t.column_id
		where t.project_id = $1 and t.issue_number = $2`, f.project, number).Scan(&s.Column, &s.Position, &s.Version, &s.NeedsAttention)
	if err != nil {
		f.t.Fatalf("task of issue %d: %v", number, err)
	}
	return s
}

// prRecord is a stored pull request record.
type prRecord struct {
	Head    string
	Draft   bool
	Linked  []int
	Refused bool
}

// record returns the stored record of pull request number, or nil when there is none.
func (f *fixture) record(number int) *prRecord {
	f.t.Helper()
	var r prRecord
	err := f.DB.QueryRow(context.Background(), `select head_sha, draft, linked_issues, refused from pull_requests where project_id = $1 and number = $2`,
		f.project, number).Scan(&r.Head, &r.Draft, &r.Linked, &r.Refused)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return &r
}

// move moves issue number's task to the end of the column with key through the move endpoint, as ada.
func (f *fixture) move(number int, key string) {
	f.t.Helper()
	var id, column int64
	var version int
	err := f.DB.QueryRow(context.Background(), `
		select t.id, t.version, c.id from tasks t join columns c on c.project_id = t.project_id
		where t.project_id = $1 and t.issue_number = $2 and c.key = $3`, f.project, number, key).Scan(&id, &version, &column)
	if err != nil {
		f.t.Fatal(err)
	}
	f.Do("POST", f.path(fmt.Sprintf("/tasks/%d/move", id)), map[string]any{"columnId": column, "version": version}, f.ada).
		Expect(http.StatusOK, "")
}

func TestReadyPullRequestStartsARun(t *testing.T) {
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			f := automated(t)
			f.exec(`update tasks set needs_attention = 'An old runner error.'`)
			n := f.pull("ada", false, 1)
			path.via(f, n)
			f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})
			if got, want := f.task(1), (taskState{Column: "testing", Position: 1, Version: 2}); got != want {
				t.Fatalf("task = %+v, want %+v: moved once, needs_attention cleared", got, want)
			}
			if labels := f.repo.Issue(1).Labels; !slices.Equal(labels, []string{"board:testing"}) {
				t.Fatalf("issue labels = %v, want the move on GitHub", labels)
			}
			if r := f.record(n); r == nil || r.Head != "a1" || r.Draft || !slices.Equal(r.Linked, []int{1}) || r.Refused {
				t.Fatalf("record = %+v", r)
			}
			if c := f.repo.Issue(n).Comments; len(c) != 0 {
				t.Fatalf("comments = %v, want none", c)
			}
		})
	}
}

func TestDraftToReadyStartsARun(t *testing.T) {
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			f := automated(t)
			n := f.pull("ada", true, 1)
			f.poll()
			f.expectRuns()
			if got := f.task(1); got.Column != "backlog" || got.Version != 1 {
				t.Fatalf("a draft moved its task: %+v", got)
			}
			if r := f.record(n); r == nil || !r.Draft {
				t.Fatalf("record = %+v, want the draft recorded", r)
			}

			f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = false })
			path.via(f, n)
			f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})
			if got := f.task(1); got.Column != "testing" || got.Version != 2 {
				t.Fatalf("task = %+v, want it moved once to Testing", got)
			}
		})
	}
}

func TestReadyToDraftCancelsTheRun(t *testing.T) {
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			f := automated(t)
			n := f.pull("ada", false, 1)
			f.poll()
			f.exec(`update test_runs set status = 'running'`)
			f.exec(`insert into workflow_executions (run_id, project_id, workflow, spec, status)
				select id, $1, v.workflow, '{}'::jsonb, v.status from test_runs,
					(values ('backend', 'running'), ('frontend', 'queued'), ('docs', 'passed')) v(workflow, status)`, f.project)

			f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = true })
			path.via(f, n)
			f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "cancelled", Reason: "The pull request went back to draft.", Finished: true})
			if got := slices.Sorted(maps.Values(f.statuses("workflow_executions"))); !slices.Equal(got, []string{"cancelled", "cancelled", "passed"}) {
				t.Fatalf("workflow executions = %v, want the running and queued ones cancelled", got)
			}
			if got := f.task(1); got.Column != "testing" || got.Version != 2 {
				t.Fatalf("task = %+v, want it left in Testing", got)
			}
		})
	}
}

func TestNewHeadWhileReadyOnlySavesTheRecord(t *testing.T) {
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			f := automated(t)
			n := f.pull("ada", false, 1)
			f.poll()
			f.repo.EditPull(n, func(p *githubtest.Pull) { p.HeadSHA = "b2" })
			path.via(f, n)
			f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})
			if r := f.record(n); r == nil || r.Head != "b2" {
				t.Fatalf("record = %+v, want head b2", r)
			}
			if got := f.task(1); got.Version != 2 {
				t.Fatalf("task = %+v, want it moved only once", got)
			}
		})
	}
}

func TestClosedPullRequestLosesItsRecord(t *testing.T) {
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			f := automated(t)
			n := f.pull("ada", false, 1)
			f.poll()
			f.repo.EditIssue(n, func(is *githubtest.Issue) { is.State = "closed" }) // closed or merged
			path.via(f, n)
			if r := f.record(n); r != nil {
				t.Fatalf("record = %+v, want it deleted", r)
			}
			f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})
		})
	}
}

func TestUntrustedPullRequestGetsOneComment(t *testing.T) {
	for _, c := range []struct {
		name    string
		open    func(f *fixture) int
		comment string
	}{
		{"fork", func(f *fixture) int {
			fork := f.GitHub.AddRepo("eve", "hello")
			return f.repo.AddPull("Fix", githubtest.Pull{Author: "ada", HeadSHA: "a1", HeadRepo: fork.ID, Closes: f.closing(1)})
		}, forkRefusal},
		{"deleted fork", func(f *fixture) int {
			return f.repo.AddPull("Fix", githubtest.Pull{Author: "ada", HeadSHA: "a1", Closes: f.closing(1)})
		}, forkRefusal},
		{"read-only author", func(f *fixture) int {
			f.member("bob", "pull")
			return f.pull("bob", false, 1)
		}, "Not tested: bob does not have write access to this repository. Automation tests only pull requests by authors who can push to it."},
		{"author without access", func(f *fixture) int {
			f.GitHub.AddUser("eve-token", 3, "eve")
			return f.pull("eve", false, 1)
		}, "Not tested: eve does not have write access to this repository. Automation tests only pull requests by authors who can push to it."},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := automated(t)
			n := c.open(f)
			f.poll()
			f.hint(n)
			f.poll()
			// Ready again later, which would start a run for a trusted pull request.
			f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = true })
			f.hint(n)
			f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = false })
			f.poll()
			f.hint(n)

			if got, want := f.repo.Issue(n).Comments, []githubtest.Comment{{Author: "ada", Body: c.comment}}; !slices.Equal(got, want) {
				t.Fatalf("comments = %+v, want %+v", got, want)
			}
			f.expectRuns()
			if got := f.task(1); got.Column != "backlog" || got.Version != 1 {
				t.Fatalf("task = %+v, want it unmoved", got)
			}
			if r := f.record(n); r == nil || !r.Refused {
				t.Fatalf("record = %+v, want it refused", r)
			}
		})
	}
}

func TestClosedTaskIsNeverTriggered(t *testing.T) {
	f := automated(t)
	f.move(1, "closed")
	before := f.task(1)
	n := f.pull("ada", false, 1)
	fork := f.GitHub.AddRepo("eve", "hello")
	m := f.repo.AddPull("Fork", githubtest.Pull{Author: "ada", HeadSHA: "a1", HeadRepo: fork.ID, Closes: f.closing(1)})
	f.poll()
	f.hint(n)
	f.hint(m)
	f.expectRuns()
	if got := f.task(1); got != before {
		t.Fatalf("task = %+v, want it left in Closed as %+v", got, before)
	}
	for _, pr := range []int{n, m} {
		if c := f.repo.Issue(pr).Comments; len(c) != 0 {
			t.Fatalf("pull request %d got comments %v, want none", pr, c)
		}
	}
}

func TestTaskInTestingStaysWhereItIs(t *testing.T) {
	f := automated(t)
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Two"}, f.ada).Expect(http.StatusCreated, "")
	f.move(1, "testing")
	f.move(2, "testing")
	before := f.task(1)
	n := f.pull("ada", false, 1)
	requests := len(f.GitHub.Requests())
	f.poll()
	if got := f.task(1); got != before {
		t.Fatalf("task = %+v, want it left as %+v", got, before)
	}
	for _, r := range f.GitHub.Requests()[requests:] {
		if strings.Contains(r, "/issues/1/") {
			t.Fatalf("the trigger wrote issue 1 on GitHub: %s", r)
		}
	}
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})
}

func TestMoreThanOneReadyPullRequestIsARunnerError(t *testing.T) {
	f := automated(t)
	first := f.pull("ada", false, 1)
	f.poll()
	second := f.pull("ada", false, 1)
	f.hint(second)
	f.poll()
	f.expectRuns(
		testRun{Issue: 1, PR: first, Head: "a1", Status: "cancelled", Reason: "A new test run started.", Finished: true},
		testRun{Issue: 1, Status: "error", Reason: ambiguous, Finished: true},
	)
	if got := f.task(1); got.Column != "testing" || got.NeedsAttention != ambiguous {
		t.Fatalf("task = %+v, want it in Testing needing attention", got)
	}
}

func TestPollUnchangedListCostsOneRequest(t *testing.T) {
	f := automated(t)
	f.pull("ada", false, 1)
	f.poll()
	runs, task := f.runs(), f.task(1)
	before := len(f.GitHub.Requests())
	f.poll()
	if got, want := f.GitHub.Requests()[before:], []string{fmt.Sprintf("GET /repositories/%d/pulls?state=open&per_page=100", f.repo.ID)}; !slices.Equal(got, want) {
		t.Fatalf("GitHub requests = %v, want %v", got, want)
	}
	f.expectRuns(runs...)
	if got := f.task(1); got != task {
		t.Fatalf("task = %+v, want it unchanged from %+v", got, task)
	}
}

func TestPollWhileOff(t *testing.T) {
	f := setup(t)
	n := f.pull("ada", false, 1)
	before := len(f.GitHub.Requests())
	f.poll()
	if got := f.GitHub.Requests()[before:]; len(got) != 0 {
		t.Fatalf("polling a project without automation called GitHub: %v", got)
	}
	if r := f.record(n); r != nil {
		t.Fatalf("record = %+v, want none", r)
	}
}

func TestPausedProjectStillRecordsTriggers(t *testing.T) {
	f := automated(t)
	f.exec(`update project_automation set paused_reason = 'ada lost write access to the repository.'`)
	n := f.pull("ada", false, 1)
	f.poll()
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})
}

func TestRejectedCredentialPauses(t *testing.T) {
	for _, route := range []string{
		"GET /repositories/{id}/pulls",
		"GET /repositories/{id}/pulls/{number}",
		"POST /graphql",
		"GET /repositories/{id}/collaborators/{login}/permission",
		"POST /repositories/{id}/issues/{number}/labels",
	} {
		t.Run(route, func(t *testing.T) {
			f := automated(t)
			n := f.pull("ada", false, 1)
			f.GitHub.Fail(route, http.StatusUnauthorized)
			if route == "GET /repositories/{id}/pulls/{number}" {
				f.hint(n) // only a delivery reads one pull request
			} else if err := f.Automation.Poll(context.Background(), f.project); !errors.Is(err, automation.ErrPaused) {
				t.Fatalf("poll err = %v, want ErrPaused", err)
			}
			if got := f.pausedReason(); got == nil || *got != "GitHub no longer accepts ada's credential." {
				t.Fatalf("paused_reason = %v", got)
			}
			f.expectRuns()
		})
	}
}

// roundTrip is an http.RoundTripper made of a function.
type roundTrip func(*http.Request) (*http.Response, error)

func (rt roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return rt(r) }

func TestPollSkipsARecordSavedMeanwhile(t *testing.T) {
	f := automated(t)
	n := f.pull("ada", false, 1)
	// Between the poll's list and its apply, the pull request goes back to draft and a delivery records that.
	next, delivered := f.Config.GitHub.HTTP.Transport, false
	f.Config.GitHub.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/graphql" && !delivered {
			delivered = true
			f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = true })
			f.hint(n)
		}
		return next.RoundTrip(r)
	})}
	f.poll()
	f.expectRuns() // the poll's view, ready, is older than the record
	if r := f.record(n); r == nil || !r.Draft {
		t.Fatalf("record = %+v, want the delivery's draft kept", r)
	}
	var etag *string
	if err := f.DB.QueryRow(context.Background(), `select pulls_etag from project_automation where project_id = $1`, f.project).Scan(&etag); err != nil || etag != nil {
		t.Fatalf("pulls_etag = %v, %v; want none saved, so the next poll lists again", etag, err)
	}

	f.poll()
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = false })
	f.poll()
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})
}
