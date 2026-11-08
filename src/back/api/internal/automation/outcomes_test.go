package automation_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/llm/llmtest"
	"gantt-stop-me/api/internal/runner"
	"gantt-stop-me/api/internal/runner/runnertest"
)

// link is where the comment of the project's latest test run of issue 1 points.
func (f *fixture) link() string {
	f.t.Helper()
	var task, run int64
	err := f.DB.QueryRow(context.Background(), `
		select t.id, max(r.id) from tasks t join test_runs r on r.task_id = t.id
		where t.project_id = $1 and t.issue_number = 1 group by t.id`, f.project).Scan(&task, &run)
	if err != nil {
		f.t.Fatal(err)
	}
	return fmt.Sprintf("%s/projects/%d/tasks/%d?run=%d", apptest.PublicURL, f.project, task, run)
}

// place puts issue 1's task in the column with key as the issue sync would, after a label change on GitHub, rather
// than through the move endpoint and its rules.
func (f *fixture) place(key string) {
	f.t.Helper()
	f.exec(`update tasks set column_id = (select id from columns where project_id = $1 and key = $2), version = version + 1
		where project_id = $1 and issue_number = 1`, f.project, key)
}

// unlinkedRun puts issue 1's task in Testing with a queued test run of no pull request.
func (f *fixture) unlinkedRun() {
	f.t.Helper()
	f.place("testing")
	f.exec(`insert into test_runs (task_id, status) select id, 'queued' from tasks where project_id = $1 and issue_number = 1`, f.project)
}

// expectComments fails the test unless issue 1's comments, all ada's, have these bodies.
func (f *fixture) expectComments(want ...string) {
	f.t.Helper()
	var got []string
	for _, c := range f.repo.Issue(1).Comments {
		if c.Author != "ada" {
			f.t.Fatalf("comment by %s, want the automation owner's: %q", c.Author, c.Body)
		}
		got = append(got, c.Body)
	}
	if !slices.Equal(got, want) {
		f.t.Fatalf("comments = %q, want %q", got, want)
	}
}

func TestRunPasses(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	f.Runner.Script(0, runnertest.Script{Log: "built", Commands: map[string]runnertest.Exit{"go test ./...": {Output: "ok board\n"}}})
	n := f.ready()
	f.work()

	f.expectRuns(pass(n))
	if got := f.task(1); got.Column != "user-check" || got.NeedsAttention != "" {
		t.Fatalf("task = %+v, want it in User Check", got)
	}
	if labels := f.repo.Issue(1).Labels; !slices.Equal(labels, []string{"board:user-check"}) {
		t.Fatalf("issue labels = %v, want the move on GitHub", labels)
	}
	f.expectResults(resultRow{1, "passed", "Exit code 0.\nok board\n", "default", false}, resultRow{2, "passed", "Exit code 0.", "default", false})
	f.expectExecutions(executionRow{"default", "passed", 1})
	var tail string
	f.DB.QueryRow(context.Background(), `select log_tail from workflow_executions`).Scan(&tail)
	if want := "built\n$ go test ./...\nok board\n$ go vet ./...\n"; tail != want {
		t.Fatalf("log tail = %q, want %q", tail, want)
	}
	f.expectCalls("prepare 1", "command 1 go test ./...", "command 1 go vet ./...", "delete 1")
	prepare := f.Runner.Requests()[0]
	want := runner.Spec{Project: f.project, Workflow: "default", Context: ".", Dockerfile: "Dockerfile", Env: map[string]string{},
		Cache: []string{}, Services: []runner.Service{}}
	if got := prepare.Spec; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("spec = %+v, want %+v", got, want)
	}
	if len(prepare.Source) == 0 {
		t.Fatal("prepare got no source")
	}
	f.expectComments("**Tests passed** on a1.\n\n" +
		"- passed: `go test ./...` (default)\n- passed: `go vet ./...` (default)\n\n" + f.link())
	if f.repo.Issue(n).Pull.Draft {
		t.Fatal("a pass turned the pull request into a draft")
	}
}

func TestRunFails(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	f.Runner.Script(0, runnertest.Script{Commands: map[string]runnertest.Exit{"go test ./...": {Code: 1, Output: "FAIL board"}}})
	n := f.ready()
	f.work()

	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "failed", Finished: true})
	if got := f.task(1); got.Column != "in-progress" || got.NeedsAttention != "" {
		t.Fatalf("task = %+v, want it in In Progress", got)
	}
	if !f.repo.Issue(n).Pull.Draft {
		t.Fatal("the pull request is still ready for review, want it back in draft")
	}
	f.expectResults(resultRow{1, "failed", "Exit code 1.\nFAIL board", "default", false}, resultRow{2, "passed", "Exit code 0.", "default", false})
	f.expectComments("**Tests failed** on a1.\n\n" +
		"- failed: `go test ./...` (default)\n- passed: `go vet ./...` (default)\n\n" + f.link())

	// The record has the draft the run made, so a drag back into Testing before the triggers read GitHub finds no ready
	// pull request, and then the triggers see no change.
	if r := f.record(n); r == nil || !r.Draft {
		t.Fatalf("record = %+v, want the draft saved", r)
	}
	failed := testRun{Issue: 1, PR: n, Head: "a1", Status: "failed", Finished: true}
	f.move(1, "testing")
	f.poll()
	f.expectRuns(failed, testRun{Issue: 1, Status: "error", Reason: "No linked pull request.", Finished: true})
}

func TestFailureTurnsThePullRequestBackToDraftWhereverTheTaskIs(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	f.Runner.Script(0, runnertest.Script{Command: runnertest.Exit{Code: 1}})
	n := f.ready()
	f.plan()
	f.place("backlog") // as a label added on GitHub would
	f.work()
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "failed", Finished: true})
	if got := f.task(1); got.Column != "backlog" {
		t.Fatalf("task = %+v, want it left in Backlog", got)
	}
	if !f.repo.Issue(n).Pull.Draft {
		t.Fatal("the pull request is still ready for review, want it back in draft")
	}
}

// twoWorkflows is a workflows file with backend and frontend.
const twoWorkflows = "workflows:\n  backend:\n    description: Go API\n  frontend:\n    description: React UI\n"

func TestRunnerErrors(t *testing.T) {
	for _, c := range []struct {
		name   string
		start  func(f *fixture) // triggers the run; the default opens a ready pull request after twoTests
		reason string           // the task's needs_attention
	}{
		{"no tests", func(f *fixture) {
			f.tests(1, "Nothing to test here.")
			f.ready()
		}, "The task has no tests."},
		{"more than 64 tests", func(f *fixture) {
			f.tests(1, "## Tests\n"+strings.Repeat("- [ ] `true`\n", 65))
			f.ready()
		}, "the Tests section has 65 tests, more than 64"},
		{"bad workflows file", func(f *fixture) {
			f.repo.AddFile("a1", ".gantt/workflows.yml", "workflows:\n  backend:\n    port: 80\n")
			f.ready()
		}, ".gantt/workflows.yml: workflows.backend.description: is required (line 2)"},
		{"a NUL in the workflows file", func(f *fixture) {
			f.repo.AddFile("a1", ".gantt/workflows.yml", "workflows:\n  backend:\n    description: Go API\n    env: {A: \"x\\0y\"}\n")
			f.ready()
		}, ".gantt/workflows.yml: workflows.backend.env.A: holds a NUL character (line 4)"},
		{"a test the database cannot store", func(f *fixture) {
			f.tests(1, "## Tests\n- [ ] `printf 'a\x00b'`\n")
			f.ready()
		}, "Test 1 holds a NUL character."},
		{"an issue by someone without write access", func(f *fixture) {
			f.member("mallory", "triage")
			f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Author = "mallory" })
			f.ready()
		}, "The issue was opened by someone without write access to the repository, so its tests do not run."},
		{"build failure", func(f *fixture) {
			f.Runner.Script(0, runnertest.Script{Failure: &runner.Failure{Stage: "build", Log: "no Dockerfile"}})
			f.ready()
		}, "The build failed. The execution's log says why."},
		{"start failure", func(f *fixture) {
			f.Runner.Script(0, runnertest.Script{Failure: &runner.Failure{Stage: "start", Log: "db never became healthy"}})
			f.ready()
		}, "The app or a sidecar did not start. The execution's log says why."},
		{"routing under 0.5", func(f *fixture) {
			f.repo.AddFile("a1", ".gantt/workflows.yml", twoWorkflows)
			f.llm.Decide(func(r llmtest.DecisionsRequest) []llm.Answer {
				var answers []llm.Answer
				for _, q := range r.Questions {
					answers = append(answers, llm.Answer{Type: "choice", Name: q.Name, Choice: "backend",
						Probabilities: []llm.Probability{{Value: "backend", Probability: 0.4}, {Value: "frontend", Probability: 0.35}}})
				}
				return answers
			})
			f.ready()
		}, "Test 1 matches no workflow well enough to run. Best guesses: backend 40%, frontend 35%. Pin it to one with [name]."},
		{"decisions call fails", func(f *fixture) {
			f.repo.AddFile("a1", ".gantt/workflows.yml", twoWorkflows)
			f.llm.Fail("POST /decisions", http.StatusInternalServerError)
			f.ready()
		}, "The decision model could not route the tests: llm: POST /decisions answered HTTP 500"},
		{"unknown pin", func(f *fixture) {
			f.tests(1, "## Tests\n- [ ] [api] `make test`\n")
			f.ready()
		}, "Test 1 is pinned to [api], which names no workflow."},
		{"timeout", func(f *fixture) {
			f.repo.AddFile("a1", ".gantt/workflows.yml", "workflows:\n  backend:\n    description: Go API\n    timeout: 500ms\n")
			f.Runner.Hold(0, "command") // until the timeout gives up on it
			f.ready()
		}, "The workflow timed out after 500ms."},
		{"no linked pull request", (*fixture).unlinkedRun, "No linked pull request."},
		{"more than one ready pull request", func(f *fixture) {
			f.pull("ada", false, 1)
			f.pull("ada", false, 1)
			f.poll()
		}, ambiguous},
		{"issue gone", func(f *fixture) {
			f.GitHub.Fail("GET /repositories/{id}/issues/{number}", http.StatusGone)
			f.ready()
		}, "GitHub refused to read the issue: github: HTTP 410."},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := withLLM(t)
			f.tests(1, twoTests)
			c.start(f)
			f.work()
			runs := f.runs()
			if got := runs[len(runs)-1]; got.Status != "error" || !got.Finished {
				t.Fatalf("test run = %+v, want a runner error", got)
			}
			if got := f.task(1); got.Column != "testing" || got.NeedsAttention != c.reason {
				t.Fatalf("task = %+v, want it in Testing needing attention: %q", got, c.reason)
			}
			comments := f.repo.Issue(1).Comments
			if len(comments) != 1 || !strings.HasPrefix(comments[0].Body, "**Tests could not finish: runner error**") ||
				!strings.Contains(comments[0].Body, c.reason) {
				t.Fatalf("comments = %+v, want one naming %q", comments, c.reason)
			}
			f.work()
			if got := f.repo.Issue(1).Comments; len(got) != 1 {
				t.Fatalf("comments = %+v, want still one", got)
			}
		})
	}
}

func TestRunnerErrorComments(t *testing.T) {
	t.Run("for the run", func(t *testing.T) {
		f := withLLM(t)
		f.tests(1, "Nothing to test here.")
		f.ready()
		f.work()
		f.expectComments("**Tests could not finish: runner error** on a1.\n\nThe task has no tests.\n\n" + f.link())
	})
	t.Run("for each test", func(t *testing.T) {
		f := withLLM(t)
		f.tests(1, twoTests)
		f.Runner.Script(0, runnertest.Script{Failure: &runner.Failure{Stage: "build", Log: "no Dockerfile"}})
		f.ready()
		f.work()
		f.expectComments("**Tests could not finish: runner error** on a1.\n\n" +
			"- error: `go test ./...` (default): The build failed. The execution's log says why.\n" +
			"- error: `go vet ./...` (default): The build failed. The execution's log says why.\n\n" + f.link())
		var tail string
		f.DB.QueryRow(context.Background(), `select log_tail from workflow_executions`).Scan(&tail)
		if tail != "no Dockerfile\n" {
			t.Fatalf("log tail = %q, want the runner's log", tail)
		}
		f.expectCalls("prepare 1", "delete 1")
	})
	t.Run("no linked pull request", func(t *testing.T) {
		f := withLLM(t)
		f.unlinkedRun()
		f.work()
		f.expectComments("**Tests could not finish: runner error**.\n\nNo linked pull request.\n\n" + f.link())
	})
}

func TestCancelledRunGetsNoComment(t *testing.T) {
	for _, c := range []struct {
		name   string
		cancel func(f *fixture, n int)
		reason string
	}{
		{"back to draft", func(f *fixture, n int) {
			f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = true })
			f.poll()
		}, "The pull request went back to draft."},
		{"automation turned off", func(f *fixture, n int) {
			f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "")
		}, "Automation was turned off."},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := withLLM(t)
			f.tests(1, twoTests)
			n := f.ready()
			h := f.Runner.Hold(0, "command")
			wait := f.workInBackground()
			<-h.Arrived()
			c.cancel(f, n)
			wait() // the worker noticed and stopped
			f.work()

			f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "cancelled", Reason: c.reason, Finished: true})
			f.expectExecutions(executionRow{"default", "cancelled", 1})
			f.expectResults(resultRow{1, "skipped", "", "default", false}, resultRow{2, "skipped", "", "default", false})
			f.expectCalls("prepare 1", "command 1 go test ./...", "delete 1")
			f.expectComments()
			if got := f.task(1); got.Column != "testing" || got.NeedsAttention != "" {
				t.Fatalf("task = %+v, want it left in Testing", got)
			}
		})
	}
}

func TestCancelSkipsQueuedExecutions(t *testing.T) {
	f := withLLM(t)
	f.tests(1, "## Tests\n- [ ] [backend] `go test ./...`\n- [ ] [frontend] `npm test`\n")
	f.repo.AddFile("a1", ".gantt/workflows.yml", twoWorkflows)
	n := f.ready()
	f.plan()
	f.repo.EditPull(n, func(p *githubtest.Pull) { p.Draft = true })
	f.poll()
	f.expectExecutions(executionRow{"backend", "cancelled", 0}, executionRow{"frontend", "cancelled", 0})
	f.expectResults(resultRow{1, "skipped", "", "backend", true}, resultRow{2, "skipped", "", "frontend", true})
}

func TestCommentIsRetriedAndNeverPostedTwice(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.GitHub.Fail("POST /repositories/{id}/issues/{number}/comments", http.StatusBadGateway)
	f.work()
	f.expectRuns(pass(n))
	f.expectComments()

	f.GitHub.Fail("POST /repositories/{id}/issues/{number}/comments", 0)
	f.work()
	f.work()
	if got := f.repo.Issue(1).Comments; len(got) != 1 {
		t.Fatalf("comments = %+v, want one", got)
	}
}

func TestCommentRefusedForGoodIsGivenUp(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	f.ready()
	f.GitHub.Fail("POST /repositories/{id}/issues/{number}/comments", http.StatusForbidden) // such as a locked issue
	f.work()
	before := len(f.GitHub.Requests())
	f.work()
	for _, r := range f.GitHub.Requests()[before:] {
		if strings.HasSuffix(r, "/comments") {
			t.Fatalf("the comment was posted again: %s", r)
		}
	}
}

func TestFinishRetriesAMoveGitHubFailed(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	n := f.ready()
	f.GitHub.Fail("POST /repositories/{id}/issues/{number}/labels", http.StatusBadGateway)
	f.work()
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "running"})
	f.expectComments()

	f.GitHub.Fail("POST /repositories/{id}/issues/{number}/labels", 0)
	f.work()
	f.expectRuns(pass(n))
	if got := f.task(1); got.Column != "user-check" {
		t.Fatalf("task = %+v, want it in User Check", got)
	}
}

func TestFinishLeavesATaskMovedOutOfTesting(t *testing.T) {
	for _, column := range []string{"backlog", "closed"} {
		t.Run(column, func(t *testing.T) {
			f := withLLM(t)
			f.tests(1, twoTests)
			n := f.ready()
			f.plan()
			f.place(column)
			before := f.task(1)
			f.work()
			f.expectRuns(pass(n))
			if got := f.task(1); got != before {
				t.Fatalf("task = %+v, want it left as %+v", got, before)
			}
		})
	}
}
