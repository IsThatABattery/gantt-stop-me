package automation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/llm/llmtest"
	"gantt-stop-me/api/internal/runner"
	"gantt-stop-me/api/internal/runner/runnertest"
)

// noChat is the runner error of an LLM check whose chat call failed, as the fake LLM fails one it has no reply for.
const noChat = "The LLM check could not finish: POST /chat/completions answered HTTP 500."

// oneCheck is an issue body with one LLM check.
const oneCheck = "## Tests\n- [ ] Signing up with an empty email answers 400\n  1. POST /api/auth/signup with no email.\n"

// withApp is a workflows file whose one workflow runs the app on port 80, so LLM checks get the browser.
const withApp = "workflows:\n  frontend:\n    description: React UI\n    port: 80\n"

// call is a chat reply calling one tool.
func call(name, arguments string) llmtest.Reply {
	return llmtest.Reply{Calls: []llm.FunctionCall{{Name: name, Arguments: arguments}}}
}

// reported is a chat reply calling report.
func reported(outcome, reason string) llmtest.Reply {
	return call("report", fmt.Sprintf(`{"outcome": %q, "reason": %q}`, outcome, reason))
}

// checkRow is a stored test result with what an LLM check keeps.
type checkRow struct {
	Status, Detail string
	Transcript     []llm.Step
	Screenshot     []byte
}

// checks returns the test results, in position order, with their transcripts and screenshots.
func (f *fixture) checks() []checkRow {
	f.t.Helper()
	rows, err := f.DB.Query(context.Background(), `select status, detail, transcript, screenshot from test_results order by run_id, position`)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var got []checkRow
	for rows.Next() {
		var c checkRow
		var transcript []byte
		if err := rows.Scan(&c.Status, &c.Detail, &transcript, &c.Screenshot); err != nil {
			f.t.Fatal(err)
		}
		if transcript != nil {
			if err := json.Unmarshal(transcript, &c.Transcript); err != nil {
				f.t.Fatal(err)
			}
		}
		got = append(got, c)
	}
	return got
}

// toolNames are the names of the tools the fake LLM's chat number i offered.
func (f *fixture) toolNames(i int) []string {
	var names []string
	for _, t := range f.llm.Chats()[i].Tools {
		names = append(names, t.Name)
	}
	return names
}

func TestLLMCheckReports(t *testing.T) {
	const check = "Signing up with an empty email answers 400"
	for _, c := range []struct {
		outcome, status, run, column string
		comment                      string // before the link
	}{
		{"pass", "passed", "passed", "user-check", "**Tests passed** on a1.\n\n- passed: " + check + " (default)\n\n"},
		{"fail", "failed", "failed", "in-progress", "**Tests failed** on a1.\n\n- failed: " + check + " (default)\n\n"},
		{"unverifiable", "manual", "passed", "user-check", "**Tests passed** on a1.\n\n- manual: " + check + " (default)\n\n" +
			"Check these by hand before approving:\n- " + check + "\n\n"},
	} {
		t.Run(c.outcome, func(t *testing.T) {
			f := withLLM(t)
			f.tests(1, oneCheck)
			f.Runner.Script(0, runnertest.Script{Commands: map[string]runnertest.Exit{"curl -si app/api": {Code: 7, Output: "no app"}}})
			f.llm.Reply(
				call("http_request", `{"method": "GET", "url": "http://app:80/"}`), // offered only with an app or a sidecar
				call("run_command", `{"command": "curl -si app/api"}`),
				reported(c.outcome, "It answered 400 invalid_email."),
			)
			n := f.ready()
			f.work()

			want := []checkRow{{Status: c.status, Detail: "It answered 400 invalid_email.", Transcript: []llm.Step{
				{Tool: "http_request", Arguments: `{"method": "GET", "url": "http://app:80/"}`, Answer: "There is no tool named http_request."},
				{Tool: "run_command", Arguments: `{"command": "curl -si app/api"}`, Answer: "Exit code 7.\nno app"},
				{Tool: "report", Arguments: fmt.Sprintf(`{"outcome": %q, "reason": "It answered 400 invalid_email."}`, c.outcome), Answer: "Reported."},
			}}}
			if got := f.checks(); !reflect.DeepEqual(got, want) {
				t.Fatalf("results = %+v, want %+v", got, want)
			}
			f.expectCalls("prepare 1", "command 1 curl -si app/api", "delete 1") // no browser without an app
			if got := f.toolNames(0); !slices.Equal(got, []string{"run_command", "report"}) {
				t.Fatalf("tools = %v", got)
			}
			if chats := f.llm.Chats(); len(chats) != 3 || chats[0].Model != llmtest.ChatModel {
				t.Fatalf("chats = %+v, want three with the chat model", chats)
			}
			f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: c.run, Finished: true})
			if got := f.task(1); got.Column != c.column {
				t.Fatalf("task = %+v, want it in %s", got, c.column)
			}
			f.expectComments(c.comment + f.link())
		})
	}
}

func TestLLMCheckRunnerErrors(t *testing.T) {
	for _, c := range []struct {
		name    string
		replies []llmtest.Reply
		detail  string
		calls   int // tool calls carried out, as the transcript keeps them
	}{
		{"no report after 50 tool calls", slices.Repeat([]llmtest.Reply{call("run_command", `{"command": "true"}`)}, 50),
			"The LLM check could not finish: no report after 50 tool calls.", 50},
		{"the chat API failing", []llmtest.Reply{call("run_command", `{"command": "true"}`)}, noChat, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := withLLM(t)
			f.tests(1, oneCheck)
			f.llm.Reply(c.replies...)
			n := f.ready()
			f.work()
			got := f.checks()
			if len(got) != 1 || got[0].Status != "error" || got[0].Detail != c.detail || len(got[0].Transcript) != c.calls {
				t.Fatalf("results = %+v, want a runner error %q after %d calls", got, c.detail, c.calls)
			}
			if commands := len(f.Runner.Requests()) - 2; commands != c.calls { // all but prepare and delete
				t.Fatalf("runner got %d commands, want %d", commands, c.calls)
			}
			f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "error", Finished: true})
			if got := f.task(1); got.Column != "testing" || got.NeedsAttention != c.detail {
				t.Fatalf("task = %+v, want it in Testing needing attention", got)
			}
		})
	}
}

func TestLLMCheckFirstMessage(t *testing.T) {
	f := withLLM(t)
	body := "Sign-up must refuse empty emails.\n\n" + oneCheck
	f.tests(1, body)
	f.repo.AddFile("a1", ".gantt/workflows.yml", "workflows:\n  frontend:\n    description: React UI\n    port: 80\n"+
		"    services:\n      db:\n        image: postgres:18\n      api:\n        context: src/back/api\n")
	f.llm.Reply(reported("pass", "Refused."))
	f.ready()
	f.work()

	chats := f.llm.Chats()
	want := "You verify one test of a pull request against its code, running in containers built from it.\n\n" +
		"The issue the pull request works on, as context:\n\n# One\n\n" + body + "\n\n" +
		"The test to verify:\n\nSigning up with an empty email answers 400\n1. POST /api/auth/signup with no email.\n\n" +
		"Workflow: frontend\nApp: http://app:80\nSidecars: db, api\n\n" +
		"Verify the test with the tools, then call report once. Use unverifiable only for what no tool can check, " +
		"such as how something looks to a person, and then say in the reason what a person should check."
	if len(chats) != 1 || len(chats[0].Messages) != 1 || chats[0].Messages[0].Role != "user" || chats[0].Messages[0].Content != want {
		t.Fatalf("first message = %+v, want %q", chats[0].Messages, want)
	}
}

func TestLLMCheckTools(t *testing.T) {
	for _, c := range []struct {
		name, file string // "" for no workflows file
		tools      []string
	}{
		{"no app or sidecar", "", []string{"run_command", "report"}},
		{"a sidecar", "workflows:\n  backend:\n    description: Go API\n    services:\n      db:\n        image: postgres:18\n",
			[]string{"run_command", "http_request", "report"}},
		{"the app", withApp,
			[]string{"run_command", "http_request", "browser_navigate", "browser_snapshot", "browser_take_screenshot", "report"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := withLLM(t)
			f.tests(1, oneCheck)
			if c.file != "" {
				f.repo.AddFile("a1", ".gantt/workflows.yml", c.file)
			}
			f.llm.Reply(reported("pass", "Refused."))
			f.ready()
			f.work()
			if got := f.toolNames(0); !slices.Equal(got, c.tools) {
				t.Fatalf("tools = %v, want %v", got, c.tools)
			}
			browsers := 0
			for _, r := range f.Runner.Requests() {
				if r.Endpoint == "browser" {
					browsers++
				}
			}
			if want := map[bool]int{true: 1, false: 0}[slices.Contains(c.tools, "browser_navigate")]; browsers != want {
				t.Fatalf("browser started %d times, want %d", browsers, want)
			}
		})
	}
}

func TestLLMCheckHTTPRequest(t *testing.T) {
	f := withLLM(t)
	f.tests(1, oneCheck)
	f.repo.AddFile("a1", ".gantt/workflows.yml", withApp)
	f.Runner.Script(0, runnertest.Script{HTTP: map[string]runner.HTTPResponse{
		"POST http://app:80/api/auth/signup": {Status: 400, Body: `{"error": {"code": "invalid_email"}}`},
	}})
	args := `{"method": "POST", "url": "http://app:80/api/auth/signup", "headers": {"Content-Type": "application/json"}, "body": "{}"}`
	f.llm.Reply(call("http_request", args), reported("pass", "400 invalid_email."))
	f.ready()
	f.work()

	var sent []runner.HTTPRequest
	for _, r := range f.Runner.Requests() {
		if r.Endpoint == "http" {
			sent = append(sent, r.HTTP)
		}
	}
	want := runner.HTTPRequest{Method: "POST", URL: "http://app:80/api/auth/signup", Headers: map[string]string{"Content-Type": "application/json"}, Body: "{}"}
	if len(sent) != 1 || !reflect.DeepEqual(sent[0], want) {
		t.Fatalf("http requests = %+v, want %+v", sent, want)
	}
	got := f.checks()
	if answer := got[0].Transcript[0].Answer; answer != `{"status":400,"headers":{},"body":"{\"error\": {\"code\": \"invalid_email\"}}"}` {
		t.Fatalf("http_request answered %s", answer)
	}
	if got[0].Status != "passed" || got[0].Screenshot != nil {
		t.Fatalf("result = %+v, want passed with no screenshot, since it used no browser tool", got[0])
	}
}

func TestLLMCheckBrowser(t *testing.T) {
	f := withLLM(t)
	f.tests(1, "## Tests\n- [ ] The sign-up page refuses an empty email\n- [ ] The board looks right on a phone\n")
	f.repo.AddFile("a1", ".gantt/workflows.yml", withApp)
	f.Runner.Script(0, runnertest.Script{Tools: map[string]string{"browser_navigate": "Page title: Sign up"}})
	f.llm.Reply(
		call("browser_navigate", `{"url": "http://app:80/signup"}`),
		call("browser_click", `{}`), // not one of the browser's tools
		reported("pass", "The page said the email is required."),
		reported("unverifiable", "Check the board on a phone."),
	)
	n := f.ready()
	f.work()

	var methods []string
	browsers := 0
	for _, r := range f.Runner.Requests() {
		switch r.Endpoint {
		case "browser":
			browsers++
		case "mcp":
			var msg struct {
				Method string
				Params struct {
					Name      string
					Arguments json.RawMessage
				}
			}
			json.Unmarshal(r.MCP, &msg)
			methods = append(methods, strings.TrimSpace(msg.Method+" "+msg.Params.Name+" "+string(msg.Params.Arguments)))
		}
	}
	if browsers != 1 {
		t.Fatalf("browser started %d times, want once for both checks", browsers)
	}
	want := []string{
		"initialize", "notifications/initialized", "tools/list",
		`tools/call browser_navigate {"url":"http://app:80/signup"}`, // the model's arguments, compacted
		`tools/call browser_take_screenshot {"type":"png"}`,          // after the first check's report; the second used no browser tool
	}
	if !slices.Equal(methods, want) {
		t.Fatalf("MCP messages = %q, want %q", methods, want)
	}
	got := f.checks()
	if len(got) != 2 || got[0].Status != "passed" || got[1].Status != "manual" || got[1].Detail != "Check the board on a phone." {
		t.Fatalf("results = %+v", got)
	}
	if steps := got[0].Transcript; len(steps) != 3 || steps[0].Answer != "Page title: Sign up" ||
		steps[1].Answer != "There is no tool named browser_click." {
		t.Fatalf("transcript = %+v", steps)
	}
	if !bytes.Equal(got[0].Screenshot, runnertest.PNG) || got[1].Screenshot != nil {
		t.Fatalf("screenshots = %d and %d bytes, want the PNG for the first check alone", len(got[0].Screenshot), len(got[1].Screenshot))
	}
	f.expectRuns(pass(n))
}

func TestLLMCheckBrowserFails(t *testing.T) {
	f := withLLM(t)
	f.tests(1, oneCheck)
	f.repo.AddFile("a1", ".gantt/workflows.yml", withApp)
	f.Runner.Script(0, runnertest.Script{Browser: &runner.Failure{Stage: "start", Log: "chromium crashed"}})
	f.ready()
	f.work()
	if got := f.checks(); len(got) != 1 || got[0].Status != "error" || got[0].Detail != "The browser did not start. The execution's log says why." {
		t.Fatalf("results = %+v", got)
	}
	var tail string
	f.DB.QueryRow(context.Background(), `select log_tail from workflow_executions`).Scan(&tail)
	if tail != "chromium crashed\n" {
		t.Fatalf("log tail = %q, want the browser's log", tail)
	}
	if chats := f.llm.Chats(); len(chats) != 0 {
		t.Fatalf("chats = %+v, want none without a browser", chats)
	}
}

func TestLLMCheckTranscriptIsCut(t *testing.T) {
	f := withLLM(t)
	f.tests(1, oneCheck)
	f.Runner.Script(0, runnertest.Script{Command: runnertest.Exit{Output: strings.Repeat("<ok>\x00", 12<<10)}})
	f.llm.Reply(
		call("run_command", `{"command": "one"}`),
		call("run_command", `{"command": "two"}`),
		reported("pass", "Fine."),
	)
	f.ready()
	f.work()
	var size int
	if err := f.DB.QueryRow(context.Background(), `select length(transcript::text) from test_results`).Scan(&size); err != nil {
		t.Fatal(err)
	}
	got := f.checks()
	if size > 64<<10 || len(got[0].Transcript) != 3 || got[0].Status != "passed" {
		t.Fatalf("transcript of %d bytes with %d steps, result %s; want at most 64 KB and every step", size, len(got[0].Transcript), got[0].Status)
	}
	if answer := got[0].Transcript[0].Answer; !strings.HasPrefix(answer, "Exit code 0.\n<ok><ok>") || !strings.HasSuffix(answer, "…") {
		t.Fatalf("answer = %.40q…, want it cut, without NUL", answer)
	}
}

func TestLLMCheckRejectedCredentialRequeues(t *testing.T) {
	f := withLLM(t)
	f.tests(1, oneCheck)
	f.ready()
	f.plan()
	f.GitHub.Fail("GET /repositories/{id}/issues/{number}", http.StatusUnauthorized) // the issue read before the check
	f.work()
	if got := f.pausedReason(); got == nil || *got != "GitHub no longer accepts ada's credential." {
		t.Fatalf("paused_reason = %v", got)
	}
	f.expectExecutions(executionRow{"default", "queued", 0})
}
