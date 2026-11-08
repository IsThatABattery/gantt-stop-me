package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"gantt-stop-me/api/internal/httpx"
)

// Client is the API's client for the runner, with one method per runner endpoint.
//
// An error is *Failure when the runner reports a failed build or start, *Error for any other answer outside 2xx,
// and otherwise a transport failure, such as the runner restarting mid-request.
type Client struct {
	URL        string // RUNNER_URL, such as http://runner:8081
	Token      string // RUNNER_TOKEN
	HTTPClient *http.Client
}

// Failure is the runner's 422: an execution's build or start failed. It is a runner error, and Log says why.
type Failure struct {
	Stage string `json:"error"` // "build" or "start"
	Log   string `json:"log"`   // the last 64 KB
}

func (f *Failure) Error() string { return "runner: " + f.Stage + " failed" }

// Error is any other answer from the runner outside 2xx, such as 400 for a bad spec or 404 for an unknown execution.
type Error struct {
	Status  int
	Code    string // the httpx error code, "" when the body was not one (such as a proxied MCP answer)
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("runner: HTTP %d %s: %s", e.Status, e.Code, e.Message)
}

// Prepare sends the spec and streams source, a GitHub .tar.gz, to the runner, which builds the images and starts
// the sidecars and the app. It returns the runner's log once they are ready.
func (c *Client) Prepare(ctx context.Context, id int64, spec Spec, source io.Reader) (string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	done := make(chan struct{})
	go func() {
		pw.CloseWithError(writeParts(mw, spec, source))
		close(done)
	}()
	// Closing the reader stops the writer if the runner answers before reading everything,
	// and waiting for it means nothing reads source once Prepare returns.
	defer func() { pr.Close(); <-done }()
	var out struct {
		Log string `json:"log"`
	}
	err := c.send(ctx, http.MethodPost, c.path(id, ""), mw.FormDataContentType(), pr, &out)
	return out.Log, err
}

// writeParts writes the spec part, then the source part, so the runner checks the spec before the tarball streams in.
func writeParts(mw *multipart.Writer, spec Spec, source io.Reader) error {
	part, err := mw.CreateFormField("spec")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(part).Encode(spec); err != nil {
		return err
	}
	if part, err = mw.CreateFormFile("source", "source.tar.gz"); err != nil {
		return err
	}
	if _, err := io.Copy(part, source); err != nil {
		return err
	}
	return mw.Close()
}

// Command runs command with sh -c in a fresh container from the test image, and returns its exit code and the
// last 64 KB of its output.
func (c *Client) Command(ctx context.Context, id int64, command string) (int, string, error) {
	var out struct {
		ExitCode int    `json:"exitCode"`
		Output   string `json:"output"`
	}
	err := c.sendJSON(ctx, http.MethodPost, c.path(id, "/commands"), map[string]string{"command": command}, &out)
	return out.ExitCode, out.Output, err
}

// HTTP makes one HTTP request to the app or a sidecar. A request that could not connect is *Error with
// code "unreachable", its message saying why.
func (c *Client) HTTP(ctx context.Context, id int64, req HTTPRequest) (HTTPResponse, error) {
	var out HTTPResponse
	err := c.sendJSON(ctx, http.MethodPost, c.path(id, "/http"), req, &out)
	return out, err
}

// Browser starts the execution's browser, and returns once its MCP endpoint accepts connections. Safe to repeat.
func (c *Client) Browser(ctx context.Context, id int64) error {
	return c.send(ctx, http.MethodPost, c.path(id, "/browser"), "", nil, nil)
}

// MCP sends one JSON-RPC message to the browser's MCP endpoint within sessionID ("" before initialize),
// and returns the answer, its Content-Type (application/json or text/event-stream) and the session id.
// An answer outside 2xx is *Error, with the answer still returned.
func (c *Client) MCP(ctx context.Context, id int64, sessionID string, body []byte) ([]byte, string, string, error) {
	req, err := c.request(ctx, http.MethodPost, c.path(id, "/mcp"), "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer res.Body.Close()
	answer, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, "", "", err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		err = answerError(res.StatusCode, answer)
	}
	return answer, res.Header.Get("Content-Type"), res.Header.Get("Mcp-Session-Id"), err
}

// Delete removes the execution's containers and network, stopping what its requests were running. Safe to repeat.
func (c *Client) Delete(ctx context.Context, id int64) error {
	return c.send(ctx, http.MethodDelete, c.path(id, ""), "", nil, nil)
}

// DeleteAll removes every execution the runner has, and any an earlier runner left.
func (c *Client) DeleteAll(ctx context.Context) error {
	return c.send(ctx, http.MethodDelete, c.URL+"/executions", "", nil, nil)
}

// path is the URL of execution id's endpoint under it.
func (c *Client) path(id int64, under string) string {
	return c.URL + "/executions/" + strconv.FormatInt(id, 10) + under
}

// request is a request to the runner carrying the token.
func (c *Client) request(ctx context.Context, method, url, contentType string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

// sendJSON is send with in as the JSON body.
func (c *Client) sendJSON(ctx context.Context, method, url string, in, out any) error {
	data, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return c.send(ctx, method, url, "application/json", bytes.NewReader(data), out)
}

// send makes one request to the runner and decodes a 2xx answer into out, if not nil.
func (c *Client) send(ctx context.Context, method, url, contentType string, body io.Reader, out any) error {
	req, err := c.request(ctx, method, url, contentType, body)
	if err != nil {
		return err
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return answerError(res.StatusCode, data)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("runner: decode %s %s: %w", method, req.URL.Path, err)
	}
	return nil
}

// answerError is the error for a runner answer outside 2xx: *Failure for a 422 with a stage, else *Error.
func answerError(status int, body []byte) error {
	var f Failure
	if status == http.StatusUnprocessableEntity && json.Unmarshal(body, &f) == nil && f.Stage != "" {
		return &f
	}
	var e struct {
		Error httpx.ErrorBody `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error.Code != "" {
		return &Error{Status: status, Code: e.Error.Code, Message: e.Error.Message}
	}
	return &Error{Status: status, Message: string(body[:min(len(body), 1<<10)])}
}
