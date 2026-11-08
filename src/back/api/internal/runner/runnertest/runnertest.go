// Package runnertest fakes the runner for API tests. It serves the runner's HTTP API, which the API drives through
// the real runner.Client, records every request, and answers from a script the test sets.
package runnertest

import (
	"bytes"
	"cmp"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/runner"
)

// Token is the only runner token the fake accepts.
const Token = "runner-test-token"

// PNG is the screenshot browser_take_screenshot answers with.
var PNG = func() []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewGray(image.Rect(0, 0, 1, 1)))
	return b.Bytes()
}()

// WebM is the recording browser_stop_video saves and the video endpoint answers: the EBML magic a WebM starts with.
var WebM = []byte("\x1a\x45\xdf\xa3 runnertest")

// Fake is a fake runner. Like the runner, it keeps executions in memory: Prepare adds one, even when it fails,
// and DELETE or Restart forgets it.
type Fake struct {
	Server *httptest.Server

	mu       sync.Mutex
	scripts  map[int64]Script // by execution id, 0 for every execution without its own
	execs    map[int64]*execution
	requests []Request
	holds    []*Hold
	stop     chan struct{} // closed when the test ends, so held requests let the server close
}

// Script is how the fake answers an execution's requests. The zero Script prepares with no log, exits 0 with no
// output for every command, answers every /http request with status 404, starts the browser, answers every
// browser tool but the screenshot and the recording ones with no text, and serves each recording.
type Script struct {
	Log      string                         // Prepare's log when it succeeds
	Failure  *runner.Failure                // set: Prepare answers 422 with it, a build or start failure
	Commands map[string]Exit                // by command text
	Command  Exit                           // for a command not in Commands
	HTTP     map[string]runner.HTTPResponse // by method and URL, such as "POST http://app:80/api/signup"
	Browser  *runner.Failure                // set: Browser answers 422 with it
	Tools    map[string]string              // the text tools/call answers, by tool name
	Video    int                            // set: the video endpoint answers this status, as an error, and keeps the recording
}

// Exit is a command's result.
type Exit struct {
	Code   int
	Output string
}

// Request is one request the fake got with the right token.
type Request struct {
	Endpoint string             // "prepare", "command", "http", "browser", "mcp", "video", "delete" or "delete all"
	ID       int64              // the execution, 0 for "delete all"
	Spec     runner.Spec        // prepare
	Source   []byte             // prepare: the tarball as it arrived
	Command  string             // command
	HTTP     runner.HTTPRequest // http
	MCP      []byte             // mcp: the JSON-RPC message
	Video    string             // video: the recording's name
}

// Hold is a request the fake keeps waiting until the test releases it.
type Hold struct {
	id       int64
	endpoint string
	arrived  chan struct{}
	release  chan struct{}
}

// execution is one prepared execution.
type execution struct {
	spec      runner.Spec
	browser   bool              // started
	session   string            // the MCP session initialize opened
	recording string            // the name of the recording browser_start_video started, "" for none
	videos    map[string][]byte // the recordings browser_stop_video saved, by name
	deleted   chan struct{}     // closed by DELETE
}

// New starts a Fake, closed when the test ends.
func New(t testing.TB) *Fake {
	f := &Fake{scripts: map[int64]Script{}, execs: map[int64]*execution{}, stop: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /executions/{id}", f.prepare)
	mux.HandleFunc("POST /executions/{id}/commands", f.command)
	mux.HandleFunc("POST /executions/{id}/http", f.request)
	mux.HandleFunc("POST /executions/{id}/browser", f.browser)
	mux.HandleFunc("/executions/{id}/mcp", f.mcp)
	mux.HandleFunc("GET /executions/{id}/browser/videos/{name}", f.video)
	mux.HandleFunc("DELETE /executions/{id}", f.delete)
	mux.HandleFunc("DELETE /executions", f.deleteAll)
	outer := http.NewServeMux()
	outer.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	outer.Handle("/", auth(mux))
	f.Server = httptest.NewServer(outer)
	t.Cleanup(func() {
		close(f.stop)
		f.Server.Close()
	})
	return f
}

// Client is a client for the fake, as the API builds one from RUNNER_URL and RUNNER_TOKEN.
func (f *Fake) Client() *runner.Client {
	return &runner.Client{URL: f.Server.URL, Token: Token, HTTPClient: f.Server.Client()}
}

// Script sets how the fake answers execution id's requests, or, with id 0, those of every execution without a
// script of its own.
func (f *Fake) Script(id int64, s Script) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts[id] = s
}

// Requests returns every request with the right token so far, in order.
func (f *Fake) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requests)
}

// Hold makes the next request to endpoint, one of Request's but "delete" and "delete all", on execution id, or on
// any execution when id is 0, wait before it answers: until Release, until the client gives up on it, or until a
// DELETE removes the execution, which answers it with 409 cancelled.
func (f *Fake) Hold(id int64, endpoint string) *Hold {
	f.mu.Lock()
	defer f.mu.Unlock()
	h := &Hold{id: id, endpoint: endpoint, arrived: make(chan struct{}), release: make(chan struct{})}
	f.holds = append(f.holds, h)
	return h
}

// Arrived is closed once the request is waiting.
func (h *Hold) Arrived() <-chan struct{} { return h.arrived }

// Release lets the request answer from its script. Call it once.
func (h *Hold) Release() { close(h.release) }

// Restart is the runner restarting: it forgets every execution and drops every open connection, so a held request
// fails on the client with a transport error, not an answer.
func (f *Fake) Restart() {
	f.mu.Lock()
	f.execs = map[int64]*execution{}
	f.mu.Unlock()
	f.Server.CloseClientConnections()
}

// auth answers 401 unless the request carries Token, as the runner does.
func auth(next http.Handler) http.Handler {
	want := []byte("Bearer " + Token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "A valid runner token is required.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// execID is the request's execution id. On a bad one it answers 404 and returns false.
func execID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such execution.")
		return 0, false
	}
	return id, true
}

// arrive records req, then holds it if a Hold matches. It returns req's execution and script, or false once the
// request is answered: 404 for an execution not prepared, 409 when a DELETE removed it while held, and no answer
// when the client gave up or the test ended.
func (f *Fake) arrive(w http.ResponseWriter, r *http.Request, req Request) (*execution, Script, bool) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	e := f.execs[req.ID]
	var h *Hold
	if i := slices.IndexFunc(f.holds, func(h *Hold) bool {
		return h.endpoint == req.Endpoint && (h.id == 0 || h.id == req.ID)
	}); e != nil && i >= 0 {
		h = f.holds[i]
		f.holds = slices.Delete(f.holds, i, i+1)
	}
	f.mu.Unlock()
	if e == nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such execution. Prepare it first.")
		return nil, Script{}, false
	}
	if h != nil {
		close(h.arrived)
		select {
		case <-h.release:
		case <-e.deleted:
			httpx.Error(w, http.StatusConflict, "cancelled", "The execution was deleted while the request ran.")
			return nil, Script{}, false
		case <-r.Context().Done():
			return nil, Script{}, false
		case <-f.stop:
			return nil, Script{}, false
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.scripts[req.ID]
	if !ok {
		s = f.scripts[0]
	}
	return e, s, true
}

// prepare is POST /executions/{id}: read the spec and the source, as the runner does, and answer from the script.
// shortcut: the spec is not checked the way the runner's unexported Spec.validate does; export it when an API test
// needs 400 bad_spec.
func (f *Fake) prepare(w http.ResponseWriter, r *http.Request) {
	id, ok := execID(w, r)
	if !ok {
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "The body must be multipart/form-data.")
		return
	}
	part, err := mr.NextPart()
	if err != nil || part.FormName() != "spec" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "The first part must be spec.")
		return
	}
	var spec runner.Spec
	if err := json.NewDecoder(io.LimitReader(part, 1<<20)).Decode(&spec); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_spec", "The spec must be valid JSON.")
		return
	}
	source, err := mr.NextPart()
	if err != nil || source.FormName() != "source" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "The second part must be source.")
		return
	}
	data, readErr := io.ReadAll(source)
	io.Copy(io.Discard, r.Body) // to the end, so the server watches the connection and a held Prepare sees the client go
	req := Request{Endpoint: "prepare", ID: id, Spec: spec, Source: data}
	f.mu.Lock()
	taken := f.execs[id] != nil
	if taken {
		f.requests = append(f.requests, req)
	} else {
		f.execs[id] = &execution{spec: spec, videos: map[string][]byte{}, deleted: make(chan struct{})}
	}
	f.mu.Unlock()
	if taken {
		httpx.Error(w, http.StatusConflict, "already_prepared", "This execution is already prepared. DELETE it first.")
		return
	}
	_, s, ok := f.arrive(w, r, req)
	switch {
	case !ok:
	case readErr != nil: // the runner fails the build on a broken upload, and keeps the execution
		httpx.JSON(w, http.StatusUnprocessableEntity, runner.Failure{Stage: "build", Log: readErr.Error()})
	case s.Failure != nil:
		httpx.JSON(w, http.StatusUnprocessableEntity, s.Failure)
	default:
		httpx.JSON(w, http.StatusOK, map[string]string{"log": s.Log})
	}
}

// command is POST /executions/{id}/commands: the script's exit code and output for the command.
func (f *Fake) command(w http.ResponseWriter, r *http.Request) {
	id, ok := execID(w, r)
	if !ok {
		return
	}
	var in struct {
		Command string `json:"command"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	_, s, ok := f.arrive(w, r, Request{Endpoint: "command", ID: id, Command: in.Command})
	if !ok {
		return
	}
	exit, found := s.Commands[in.Command]
	if !found {
		exit = s.Command
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"exitCode": exit.Code, "output": exit.Output})
}

// request is POST /executions/{id}/http: the script's answer for the method and URL.
func (f *Fake) request(w http.ResponseWriter, r *http.Request) {
	id, ok := execID(w, r)
	if !ok {
		return
	}
	var in runner.HTTPRequest
	if !httpx.Decode(w, r, &in) {
		return
	}
	_, s, ok := f.arrive(w, r, Request{Endpoint: "http", ID: id, HTTP: in})
	if !ok {
		return
	}
	res, found := s.HTTP[cmp.Or(in.Method, http.MethodGet)+" "+in.URL]
	if !found {
		res = runner.HTTPResponse{Status: http.StatusNotFound}
	}
	if res.Headers == nil {
		res.Headers = http.Header{}
	}
	httpx.JSON(w, http.StatusOK, res)
}

// browser is POST /executions/{id}/browser: 409 no_port without a port, as the runner answers, else the script's.
func (f *Fake) browser(w http.ResponseWriter, r *http.Request) {
	id, ok := execID(w, r)
	if !ok {
		return
	}
	e, s, ok := f.arrive(w, r, Request{Endpoint: "browser", ID: id})
	switch {
	case !ok:
	case e.spec.Port == nil:
		httpx.Error(w, http.StatusConflict, "no_port", "The workflow has no port, so it gets no browser.")
	case s.Browser != nil:
		httpx.JSON(w, http.StatusUnprocessableEntity, s.Browser)
	default:
		f.mu.Lock()
		e.browser = true
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}
}

// tools are the browser tools tools/list offers, a few of the Playwright MCP's. The last three are --caps=devtools
// tools, which the API must not offer the model.
var tools = []map[string]any{
	{"name": "browser_navigate", "description": "Navigate to a URL",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"url": map[string]string{"type": "string"}}, "required": []string{"url"}}},
	{"name": "browser_snapshot", "description": "Capture accessibility snapshot of the current page",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
	{"name": "browser_take_screenshot", "description": "Take a screenshot of the current page",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
	{"name": "browser_annotate", "description": "Open the Playwright Dashboard in annotation mode for the current page and wait for the user to draw annotations.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
	{"name": "browser_start_video", "description": "Start video recording",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"filename": map[string]string{"type": "string"}}}},
	{"name": "browser_stop_video", "description": "Stop video recording",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
}

// mcp is /executions/{id}/mcp: a small MCP server in place of the browser's, answering requests as server-sent
// events the way the Playwright MCP does. initialize opens a session, tools/list offers tools, and tools/call answers
// the script's text, for browser_take_screenshot an image item holding PNG, and for browser_start_video and
// browser_stop_video what the Playwright MCP says, the stop saving WebM under the name of the start's filename.
func (f *Fake) mcp(w http.ResponseWriter, r *http.Request) {
	id, ok := execID(w, r)
	if !ok {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	e, s, ok := f.arrive(w, r, Request{Endpoint: "mcp", ID: id, MCP: body})
	if !ok {
		return
	}
	f.mu.Lock()
	started, session := e.browser, e.session
	f.mu.Unlock()
	if !started {
		httpx.Error(w, http.StatusConflict, "no_browser", "Start the browser first.")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var msg struct {
		ID     json.RawMessage `json:"id"` // absent on a notification
		Method string          `json:"method"`
		Params struct {
			ProtocolVersion string `json:"protocolVersion"`
			Name            string `json:"name"`
			Arguments       struct {
				Filename string `json:"filename"`
			} `json:"arguments"`
		} `json:"params"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		refuse(w, http.StatusBadRequest, -32700, "Parse error")
		return
	}
	switch {
	case msg.Method == "initialize":
		session = "session-" + strconv.FormatInt(id, 10)
		f.mu.Lock()
		e.session = session
		f.mu.Unlock()
		w.Header().Set("Mcp-Session-Id", session)
		event(w, msg.ID, "result", map[string]any{"protocolVersion": cmp.Or(msg.Params.ProtocolVersion, "2025-06-18"),
			"capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "Playwright", "version": "fake"}})
	case r.Header.Get("Mcp-Session-Id") == "":
		refuse(w, http.StatusBadRequest, -32000, "Bad Request: Mcp-Session-Id header is required")
	case r.Header.Get("Mcp-Session-Id") != session:
		refuse(w, http.StatusNotFound, -32001, "Session not found")
	case msg.ID == nil:
		w.WriteHeader(http.StatusAccepted)
	case msg.Method == "tools/list":
		event(w, msg.ID, "result", map[string]any{"tools": tools})
	case msg.Method == "tools/call" && msg.Params.Name == "browser_take_screenshot":
		event(w, msg.ID, "result", map[string]any{"content": []map[string]string{
			{"type": "text", "text": "Took a screenshot of the current page."},
			{"type": "image", "data": base64.StdEncoding.EncodeToString(PNG), "mimeType": "image/png"},
		}})
	case msg.Method == "tools/call" && msg.Params.Name == "browser_start_video":
		f.mu.Lock()
		recording := e.recording != ""
		if !recording {
			e.recording = cmp.Or(strings.TrimSuffix(path.Base(msg.Params.Arguments.Filename), ".webm"), "video")
		}
		f.mu.Unlock()
		if recording {
			event(w, msg.ID, "result", map[string]any{"isError": true,
				"content": []map[string]string{{"type": "text", "text": "### Error\nError: Video recording has already been started."}}})
			break
		}
		event(w, msg.ID, "result", map[string]any{"content": []map[string]string{{"type": "text", "text": "### Result\nVideo recording started."}}})
	case msg.Method == "tools/call" && msg.Params.Name == "browser_stop_video":
		f.mu.Lock()
		name := e.recording
		if name != "" {
			e.videos[name], e.recording = WebM, ""
		}
		f.mu.Unlock()
		text := "### Result\nNo videos were recorded."
		if name != "" {
			text = "### Result\n- [Video](" + runner.VideoFile(name) + ")"
		}
		event(w, msg.ID, "result", map[string]any{"content": []map[string]string{{"type": "text", "text": text}}})
	case msg.Method == "tools/call" && slices.ContainsFunc(tools, func(t map[string]any) bool { return t["name"] == msg.Params.Name }):
		event(w, msg.ID, "result", map[string]any{"content": []map[string]string{{"type": "text", "text": s.Tools[msg.Params.Name]}}})
	case msg.Method == "tools/call":
		event(w, msg.ID, "error", map[string]any{"code": -32602, "message": fmt.Sprintf("Tool %s not found", msg.Params.Name)})
	default:
		event(w, msg.ID, "error", map[string]any{"code": -32601, "message": "Method not found"})
	}
}

// event answers a JSON-RPC request with one server-sent event holding its result or error.
func event(w http.ResponseWriter, id json.RawMessage, field string, value any) {
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, field: value})
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
}

// refuse answers an MCP message the server will not take, as plain JSON with an HTTP error status.
func refuse(w http.ResponseWriter, status, code int, message string) {
	httpx.JSON(w, status, map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": code, "message": message}})
}

// video is GET /executions/{id}/browser/videos/{name}: the script's status, or the recording browser_stop_video saved
// under name, which it then forgets, as the runner deletes it, or 404 when there is none.
// shortcut: the name is not checked the way the runner's videoName does; check it when an API test needs 400 bad_name.
func (f *Fake) video(w http.ResponseWriter, r *http.Request) {
	id, ok := execID(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	e, s, ok := f.arrive(w, r, Request{Endpoint: "video", ID: id, Video: name})
	if !ok {
		return
	}
	if s.Video != 0 {
		httpx.Error(w, s.Video, "runnertest", "The script fails the video endpoint.")
		return
	}
	f.mu.Lock()
	webm, found := e.videos[name]
	delete(e.videos, name)
	f.mu.Unlock()
	if !found {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such recording.")
		return
	}
	w.Header().Set("Content-Type", "video/webm")
	w.Write(webm)
}

// delete is DELETE /executions/{id}: forget the execution, ending its held requests. Safe to repeat.
func (f *Fake) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := execID(w, r)
	if !ok {
		return
	}
	f.mu.Lock()
	f.requests = append(f.requests, Request{Endpoint: "delete", ID: id})
	if e := f.execs[id]; e != nil {
		close(e.deleted)
		delete(f.execs, id)
	}
	f.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// deleteAll is DELETE /executions: the same for every execution.
func (f *Fake) deleteAll(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, Request{Endpoint: "delete all"})
	for _, e := range f.execs {
		close(e.deleted)
	}
	f.execs = map[int64]*execution{}
	f.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
