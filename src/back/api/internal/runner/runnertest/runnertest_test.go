package runnertest_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/runner"
	"gantt-stop-me/api/internal/runner/runnertest"
)

// spec is a workflow with an app on port 80.
func spec() runner.Spec {
	port := 80
	return runner.Spec{Project: 1, Workflow: "frontend", Context: ".", Dockerfile: "Dockerfile", Port: &port}
}

// isError reports whether err is the runner answering status with code.
func isError(err error, status int, code string) bool {
	e := (*runner.Error)(nil)
	return errors.As(err, &e) && e.Status == status && e.Code == code
}

// mcp sends one JSON-RPC message and returns the answer's result or error, as JSON.
func mcp(t *testing.T, c *runner.Client, session, body string) (string, string) {
	t.Helper()
	answer, contentType, gotSession, err := c.MCP(t.Context(), 1, session, "2025-06-18", []byte(body))
	if err != nil {
		t.Fatalf("MCP(%s) = %v", body, err)
	}
	_, data, found := strings.Cut(string(answer), "event: message\ndata: ")
	if contentType != "text/event-stream" || !found {
		t.Fatalf("MCP(%s) = %s %q, want one server-sent event", body, contentType, answer)
	}
	var msg struct {
		Result, Error json.RawMessage
	}
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatal(err)
	}
	return gotSession, string(msg.Result) + string(msg.Error)
}

func TestFake(t *testing.T) {
	f := runnertest.New(t)
	f.Script(0, runnertest.Script{
		Log:      "built",
		Commands: map[string]runnertest.Exit{"go test ./...": {Code: 1, Output: "FAIL"}},
		HTTP:     map[string]runner.HTTPResponse{"POST http://app:80/api/signup": {Status: 400, Body: "invalid_email"}},
		Tools:    map[string]string{"browser_navigate": "Navigated to http://app:80/"},
	})
	c := f.Client()
	ctx := t.Context()

	if _, _, err := (&runner.Client{URL: c.URL, Token: "wrong", HTTPClient: c.HTTPClient}).Command(ctx, 1, "true"); !isError(err, 401, "unauthorized") {
		t.Errorf("a wrong token: %v, want 401", err)
	}
	if _, _, err := c.Command(ctx, 1, "true"); !isError(err, 404, "not_found") {
		t.Errorf("Command before Prepare = %v, want 404", err)
	}
	if log, err := c.Prepare(ctx, 1, spec(), strings.NewReader("tarball")); err != nil || log != "built" {
		t.Fatalf("Prepare = %q, %v", log, err)
	}
	if _, err := c.Prepare(ctx, 1, spec(), strings.NewReader("tarball")); !isError(err, 409, "already_prepared") {
		t.Errorf("Prepare again = %v, want 409", err)
	}
	if code, out, err := c.Command(ctx, 1, "go test ./..."); err != nil || code != 1 || out != "FAIL" {
		t.Errorf("a scripted command = %d %q, %v", code, out, err)
	}
	if code, out, err := c.Command(ctx, 1, "true"); err != nil || code != 0 || out != "" {
		t.Errorf("another command = %d %q, %v", code, out, err)
	}
	req := runner.HTTPRequest{Method: "POST", URL: "http://app:80/api/signup", Body: `{"email": ""}`}
	if res, err := c.HTTP(ctx, 1, req); err != nil || res.Status != 400 || res.Body != "invalid_email" {
		t.Errorf("a scripted request = %+v, %v", res, err)
	}
	if res, err := c.HTTP(ctx, 1, runner.HTTPRequest{URL: "http://app:80/"}); err != nil || res.Status != 404 {
		t.Errorf("another request = %+v, %v", res, err)
	}

	if _, _, _, err := c.MCP(ctx, 1, "", "", []byte(`{}`)); !isError(err, 409, "no_browser") {
		t.Errorf("MCP before Browser = %v, want 409 no_browser", err)
	}
	if err := c.Browser(ctx, 1); err != nil {
		t.Fatal(err)
	}
	session, result := mcp(t, c, "", `{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18"}}`)
	if session != "session-1" || !strings.Contains(result, `"protocolVersion":"2025-06-18"`) {
		t.Errorf("initialize = session %q, %s", session, result)
	}
	if answer, _, _, err := c.MCP(ctx, 1, session, "", []byte(`{"jsonrpc": "2.0", "method": "notifications/initialized"}`)); err != nil || len(answer) != 0 {
		t.Errorf("a notification = %q, %v", answer, err)
	}
	if _, result := mcp(t, c, session, `{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}`); !strings.Contains(result, `"name":"browser_take_screenshot"`) {
		t.Errorf("tools/list = %s", result)
	}
	if _, result := mcp(t, c, session, `{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "browser_navigate", "arguments": {"url": "http://app:80/"}}}`); result != `{"content":[{"text":"Navigated to http://app:80/","type":"text"}]}` {
		t.Errorf("browser_navigate = %s", result)
	}
	_, result = mcp(t, c, session, `{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {"name": "browser_take_screenshot"}}`)
	var shot struct {
		Content []struct{ Type, Data, MimeType string }
	}
	json.Unmarshal([]byte(result), &shot)
	if i := slices.IndexFunc(shot.Content, func(c struct{ Type, Data, MimeType string }) bool { return c.Type == "image" }); i < 0 ||
		shot.Content[i].MimeType != "image/png" || shot.Content[i].Data != base64.StdEncoding.EncodeToString(runnertest.PNG) {
		t.Errorf("browser_take_screenshot = %s, want an image item of PNG", result)
	}
	if _, result := mcp(t, c, session, `{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": {"name": "nope"}}`); !strings.Contains(result, `"code":-32602`) {
		t.Errorf("an unknown tool = %s, want an error", result)
	}
	if _, _, _, err := c.MCP(ctx, 1, "", "", []byte(`{"jsonrpc": "2.0", "id": 6, "method": "tools/list"}`)); !isError(err, 400, "") {
		t.Errorf("MCP without a session = %v, want 400", err)
	}

	if err := c.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Command(ctx, 1, "true"); !isError(err, 404, "not_found") {
		t.Errorf("Command after Delete = %v, want 404", err)
	}
	if err := c.DeleteAll(ctx); err != nil {
		t.Fatal(err)
	}

	var endpoints []string
	for _, r := range f.Requests() {
		endpoints = append(endpoints, r.Endpoint)
	}
	want := []string{"command", "prepare", "prepare", "command", "command", "http", "http", "mcp", "browser",
		"mcp", "mcp", "mcp", "mcp", "mcp", "mcp", "mcp", "delete", "command", "delete all"}
	if !slices.Equal(endpoints, want) {
		t.Errorf("endpoints = %v, want %v", endpoints, want)
	}
	reqs := f.Requests()
	if p := reqs[1]; p.ID != 1 || p.Spec.Workflow != "frontend" || *p.Spec.Port != 80 || string(p.Source) != "tarball" {
		t.Errorf("Prepare was recorded as %+v", p)
	}
	if reqs[3].Command != "go test ./..." || reqs[5].HTTP.Body != `{"email": ""}` || !bytes.Contains(reqs[12].MCP, []byte("browser_navigate")) {
		t.Errorf("requests were recorded as %+v", reqs)
	}
}

func TestFakeScriptsPerExecution(t *testing.T) {
	f := runnertest.New(t)
	f.Script(0, runnertest.Script{Log: "everyone's"})
	f.Script(2, runnertest.Script{
		Failure: &runner.Failure{Stage: "build", Log: "no Dockerfile"},
		Browser: &runner.Failure{Stage: "start", Log: "no browser"},
	})
	c := f.Client()
	ctx := t.Context()
	if log, err := c.Prepare(ctx, 1, spec(), strings.NewReader("")); err != nil || log != "everyone's" {
		t.Errorf("Prepare(1) = %q, %v", log, err)
	}
	fail := (*runner.Failure)(nil)
	if _, err := c.Prepare(ctx, 2, spec(), strings.NewReader("")); !errors.As(err, &fail) || *fail != (runner.Failure{Stage: "build", Log: "no Dockerfile"}) {
		t.Errorf("Prepare(2) = %v, want its build failure", err)
	}
	// A failed Prepare keeps the execution, as the runner does.
	if err := c.Browser(ctx, 2); !errors.As(err, &fail) || fail.Stage != "start" {
		t.Errorf("Browser(2) = %v, want its start failure", err)
	}
	noPort := spec()
	noPort.Port = nil
	if _, err := c.Prepare(ctx, 3, noPort, strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	if err := c.Browser(ctx, 3); !isError(err, 409, "no_port") {
		t.Errorf("Browser without a port = %v, want 409 no_port", err)
	}
}

func TestFakeHold(t *testing.T) {
	f := runnertest.New(t)
	f.Script(0, runnertest.Script{Command: runnertest.Exit{Output: "done"}})
	c := f.Client()
	if _, err := c.Prepare(t.Context(), 1, spec(), strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	type answer struct {
		out string
		err error
	}
	command := func(ctx context.Context) chan answer {
		got := make(chan answer, 1)
		go func() {
			_, out, err := c.Command(ctx, 1, "sleep")
			got <- answer{out, err}
		}()
		return got
	}

	// Released, it answers from the script.
	h := f.Hold(1, "command")
	got := command(t.Context())
	<-h.Arrived()
	h.Release()
	if a := <-got; a.err != nil || a.out != "done" {
		t.Errorf("a released command = %+v", a)
	}

	// The API giving up on it.
	h = f.Hold(0, "command")
	ctx, cancel := context.WithCancel(t.Context())
	got = command(ctx)
	<-h.Arrived()
	cancel()
	if a := <-got; !errors.Is(a.err, context.Canceled) {
		t.Errorf("a cancelled command = %+v", a)
	}
	// The hold is used up.
	if _, out, err := c.Command(t.Context(), 1, "sleep"); err != nil || out != "done" {
		t.Errorf("the next command = %q, %v", out, err)
	}

	// A DELETE ending it.
	h = f.Hold(1, "command")
	got = command(t.Context())
	<-h.Arrived()
	if err := c.Delete(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if a := <-got; !isError(a.err, 409, "cancelled") {
		t.Errorf("a command deleted under = %+v, want 409 cancelled", a)
	}

	// A hold on one execution leaves another's requests alone.
	f.Hold(5, "prepare")
	if _, err := c.Prepare(t.Context(), 1, spec(), strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
}

func TestFakeRestart(t *testing.T) {
	f := runnertest.New(t)
	c := f.Client()
	if _, err := c.Prepare(t.Context(), 1, spec(), strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	h := f.Hold(1, "command")
	got := make(chan error, 1)
	go func() {
		_, _, err := c.Command(t.Context(), 1, "sleep")
		got <- err
	}()
	<-h.Arrived()
	f.Restart()
	err := <-got
	if e, fail := (*runner.Error)(nil), (*runner.Failure)(nil); err == nil || errors.As(err, &e) || errors.As(err, &fail) {
		t.Fatalf("a command when the runner restarts = %v, want a transport failure", err)
	}
	// The restarted runner has forgotten the execution, and prepares it anew.
	if _, _, err := c.Command(t.Context(), 1, "true"); !isError(err, 404, "not_found") {
		t.Errorf("Command after the restart = %v, want 404", err)
	}
	if _, err := c.Prepare(t.Context(), 1, spec(), strings.NewReader("")); err != nil {
		t.Errorf("Prepare after the restart = %v", err)
	}
	res, err := http.Get(f.Server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Errorf("GET /health = %d", res.StatusCode)
	}
}
