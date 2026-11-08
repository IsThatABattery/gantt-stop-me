package automation_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/llm/llmtest"
)

// twoTests is an issue body with two scripts.
const twoTests = "## Tests\n- [ ] `go test ./...`\n- [ ] `go vet ./...`\n"

// withLLM is a fixture with automation on, ada its owner, and the fake LLM as both her decision and chat model.
func withLLM(t *testing.T, configure ...func(*app.Config)) *fixture {
	f := setup(t, configure...)
	f.llm = llmtest.New(t)
	var provider int64
	err := f.DB.QueryRow(context.Background(), `
		insert into providers (user_id, name, base_url, key_enc, models) values ($1, 'Fake', $2, $3, $4) returning id`,
		f.adaID, f.llm.URL, f.Config.Secrets.Seal([]byte(f.llm.Key)), f.llm.Models).Scan(&provider)
	if err != nil {
		t.Fatal(err)
	}
	f.Do("PUT", f.path("/automation"), map[string]any{
		"decisionProviderId": provider, "decisionModel": llmtest.DecisionModel, "chatProviderId": provider, "chatModel": llmtest.ChatModel,
	}, f.ada).Expect(http.StatusOK, "")
	return f
}

// tests sets the body of issue number on GitHub.
func (f *fixture) tests(number int, body string) {
	f.repo.EditIssue(number, func(is *githubtest.Issue) { is.Body = body })
}

// ready opens a ready pull request by ada closing issue 1, with head "a1", polls so it starts a test run, and returns
// the pull request's number.
func (f *fixture) ready() int {
	f.t.Helper()
	n := f.pull("ada", false, 1)
	f.poll()
	return n
}

// work runs one pass of the queue worker.
func (f *fixture) work() { f.Automation.Work(context.Background()) }

// workInBackground starts one pass of the queue worker, for a test that acts while it drives an execution, and
// returns a function that waits for the pass to end.
func (f *fixture) workInBackground() (wait func()) {
	done := make(chan struct{})
	go func() {
		f.work()
		close(done)
	}()
	return func() { <-done }
}

// plan runs one pass of the queue worker that claims nothing, so the queued test runs are planned and no more.
func (f *fixture) plan() {
	limit := f.Automation.MaxConcurrentWorkflows
	f.Automation.MaxConcurrentWorkflows = 0
	f.work()
	f.Automation.MaxConcurrentWorkflows = limit
}

// resultRow is a stored test result, with "" for no workflow.
type resultRow struct {
	Position int
	Status   string
	Detail   string
	Workflow string
	Pinned   bool
}

// results returns the project's test results, by run and position.
func (f *fixture) results() []resultRow {
	f.t.Helper()
	rows, _ := f.DB.Query(context.Background(), `
		select x.position, x.status, x.detail, coalesce(e.workflow, ''), x.pinned
		from test_results x left join workflow_executions e on e.id = x.execution_id order by x.run_id, x.position`)
	results, err := pgx.CollectRows(rows, pgx.RowToStructByPos[resultRow])
	if err != nil {
		f.t.Fatal(err)
	}
	return results
}

// expectResults fails the test unless the project's test results are want.
func (f *fixture) expectResults(want ...resultRow) {
	f.t.Helper()
	if got := f.results(); !slices.Equal(got, want) {
		f.t.Fatalf("test results = %+v, want %+v", got, want)
	}
}

// executionRow is a stored workflow execution.
type executionRow struct {
	Workflow string
	Status   string
	Attempts int
}

// executions returns the workflow executions, in id order.
func (f *fixture) executions() []executionRow {
	f.t.Helper()
	rows, _ := f.DB.Query(context.Background(), `select workflow, status, attempts from workflow_executions order by id`)
	executions, err := pgx.CollectRows(rows, pgx.RowToStructByPos[executionRow])
	if err != nil {
		f.t.Fatal(err)
	}
	return executions
}

// expectExecutions fails the test unless the workflow executions are want.
func (f *fixture) expectExecutions(want ...executionRow) {
	f.t.Helper()
	if got := f.executions(); !slices.Equal(got, want) {
		f.t.Fatalf("workflow executions = %+v, want %+v", got, want)
	}
}

// calls are the fake runner's requests so far, as "endpoint id", with a command's text after it.
func (f *fixture) calls() []string {
	var calls []string
	for _, r := range f.Runner.Requests() {
		c := fmt.Sprintf("%s %d", r.Endpoint, r.ID)
		if r.Command != "" {
			c += " " + r.Command
		}
		calls = append(calls, c)
	}
	return calls
}

// expectCalls fails the test unless the fake runner's requests so far are want.
func (f *fixture) expectCalls(want ...string) {
	f.t.Helper()
	if got := f.calls(); !slices.Equal(got, want) {
		f.t.Fatalf("runner requests = %q, want %q", got, want)
	}
}

// pass is a passed run of issue 1 by pull request n at head a1.
func pass(n int) testRun {
	return testRun{Issue: 1, PR: n, Head: "a1", Status: "passed", Finished: true}
}

func TestQueueRunsAtMostMaxConcurrentWorkflows(t *testing.T) {
	const file = "workflows:\n  backend:\n    description: Go API\n  frontend:\n    description: React UI\n"
	const body = "## Tests\n- [ ] [backend] `go test ./...`\n- [ ] [frontend] `npm test`\n"
	t.Run("one", func(t *testing.T) {
		f := withLLM(t)
		f.tests(1, body)
		f.repo.AddFile("a1", ".gantt/workflows.yml", file)
		n := f.ready()
		f.work()
		f.expectExecutions(executionRow{"backend", "passed", 1}, executionRow{"frontend", "queued", 0})
		f.expectCalls("prepare 1", "command 1 go test ./...", "delete 1")
		f.work()
		f.expectExecutions(executionRow{"backend", "passed", 1}, executionRow{"frontend", "passed", 1})
		f.expectRuns(pass(n))
	})
	t.Run("two at once", func(t *testing.T) {
		f := withLLM(t, func(c *app.Config) { c.MaxConcurrentWorkflows = 2 })
		f.tests(1, body)
		f.repo.AddFile("a1", ".gantt/workflows.yml", file)
		n := f.ready()
		first, second := f.Runner.Hold(0, "prepare"), f.Runner.Hold(0, "prepare")
		wait := f.workInBackground()
		<-first.Arrived()
		<-second.Arrived() // both prepare before either answers: two run at once
		first.Release()
		second.Release()
		wait()
		f.expectExecutions(executionRow{"backend", "passed", 1}, executionRow{"frontend", "passed", 1})
		f.expectRuns(pass(n))
	})
}

func TestQueueNeverRunsAProjectsWorkflowTwiceAtOnce(t *testing.T) {
	f := withLLM(t, func(c *app.Config) { c.MaxConcurrentWorkflows = 2 })
	f.Do("POST", f.path("/tasks"), map[string]string{"title": "Two"}, f.ada).Expect(http.StatusCreated, "")
	f.tests(1, "## Tests\n- [ ] `true`\n")
	f.tests(2, "## Tests\n- [ ] `true`\n")
	one, two := f.pull("ada", false, 1), f.pull("ada", false, 2)
	f.poll()
	f.work()
	f.expectExecutions(executionRow{"default", "passed", 1}, executionRow{"default", "queued", 0})
	f.work()
	f.expectExecutions(executionRow{"default", "passed", 1}, executionRow{"default", "passed", 1})
	f.expectRuns(pass(one), testRun{Issue: 2, PR: two, Head: "a1", Status: "passed", Finished: true})
}

func TestQueueSkipsPausedProjects(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.exec(`update project_automation set paused_reason = 'ada lost write access to the repository.'`)
	f.work()
	f.expectResults() // not even planned
	f.exec(`update project_automation set paused_reason = null`)
	f.plan()
	f.exec(`update project_automation set paused_reason = 'ada lost write access to the repository.'`)
	f.work()
	f.expectExecutions(executionRow{"default", "queued", 0})
	f.expectCalls()
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})

	f.enable(f.ada, f.provider).Expect(http.StatusOK, "") // a takeover unpauses
	f.work()
	f.expectRuns(pass(n))
}

func TestClaimChecksTheOwner(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	f.ready()
	f.plan()
	f.repo.Grant(f.token, "pull")
	f.expire()
	f.work()
	if got := f.pausedReason(); got == nil || *got != "ada lost write access to the repository." {
		t.Fatalf("paused_reason = %v", got)
	}
	f.expectExecutions(executionRow{"default", "queued", 0}) // put back, the attempt not counted
	f.expectCalls()
}

func TestRejectedCredentialRequeuesTheExecution(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.GitHub.Fail("GET /repositories/{id}/tarball/{sha}", http.StatusUnauthorized)
	f.work()
	if got := f.pausedReason(); got == nil || *got != "GitHub no longer accepts ada's credential." {
		t.Fatalf("paused_reason = %v", got)
	}
	f.expectExecutions(executionRow{"default", "queued", 0})
	f.expectCalls("delete 1")
	f.expectResults(resultRow{1, "pending", "", "default", false}, resultRow{2, "pending", "", "default", false})

	f.GitHub.Fail("GET /repositories/{id}/tarball/{sha}", 0)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	f.work()
	f.expectRuns(pass(n))
}

func TestRestartRecovery(t *testing.T) {
	for _, c := range []struct {
		name     string
		attempts int // before the API stopped
		want     executionRow
	}{
		{"once", 1, executionRow{"default", "queued", 1}},
		{"twice", 2, executionRow{"default", "error", 2}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := withLLM(t)
			f.tests(1, twoTests)
			n := f.ready()
			f.plan()
			// The API stopped while it ran the execution's first test.
			f.exec(`update test_runs set status = 'running'`)
			f.exec(`update workflow_executions set status = 'running', started_at = now(), attempts = $1`, c.attempts)
			f.exec(`update test_results set status = 'passed', detail = 'Exit code 0.' where position = 1`)

			if err := f.Automation.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			f.expectCalls("delete all 0")
			f.expectExecutions(c.want)
			f.work()
			if c.name == "once" {
				f.expectCalls("delete all 0", "prepare 1", "command 1 go vet ./...", "delete 1")
				f.expectRuns(pass(n))
				f.expectResults(resultRow{1, "passed", "Exit code 0.", "default", false}, resultRow{2, "passed", "Exit code 0.", "default", false})
				return
			}
			f.expectCalls("delete all 0")
			f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "error", Finished: true})
			f.expectResults(resultRow{1, "passed", "Exit code 0.", "default", false}, resultRow{2, "error", "The runner was interrupted twice.", "default", false})
			if got := f.task(1); got.Column != "testing" || got.NeedsAttention != "The runner was interrupted twice." {
				t.Fatalf("task = %+v", got)
			}
		})
	}
}

func TestRecoverWhileOff(t *testing.T) {
	f := setup(t, func(c *app.Config) { c.RunnerURL = "" })
	if err := f.Automation.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.expectCalls()
	f.Automation.RunWorker(context.Background()) // returns at once instead of working
}

func TestRunnerRestartRequeuesOnce(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	restart := func() {
		h := f.Runner.Hold(0, "command")
		wait := f.workInBackground()
		<-h.Arrived()
		f.Runner.Restart()
		wait()
	}
	restart()
	f.expectExecutions(executionRow{"default", "queued", 1})
	f.expectResults(resultRow{1, "pending", "", "default", false}, resultRow{2, "pending", "", "default", false})
	f.expectCalls("prepare 1", "command 1 go test ./...", "delete 1")
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "running"})

	restart()
	f.expectExecutions(executionRow{"default", "error", 2})
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "error", Finished: true})
	f.expectResults(
		resultRow{1, "error", "The runner was interrupted twice.", "default", false},
		resultRow{2, "error", "The runner was interrupted twice.", "default", false},
	)
}

func TestRunnerRestartThenPasses(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	h := f.Runner.Hold(0, "command")
	wait := f.workInBackground()
	<-h.Arrived()
	f.Runner.Restart()
	wait()
	f.work()
	f.expectExecutions(executionRow{"default", "passed", 2})
	f.expectRuns(pass(n))
	if got := f.task(1); got.Column != "user-check" {
		t.Fatalf("task = %+v, want it in User Check", got)
	}
}
