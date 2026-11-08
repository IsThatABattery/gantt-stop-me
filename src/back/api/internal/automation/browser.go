package automation

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/runner"
)

// mcpVersion is the MCP protocol version the API asks the browser for.
const mcpVersion = "2025-06-18"

// browser is a workflow execution's MCP session with its browser, which every LLM check of the execution shares.
type browser struct {
	runner  *runner.Client
	id      int64      // the execution's
	session string     // Mcp-Session-Id, from initialize
	version string     // Mcp-Protocol-Version, as initialize agreed it
	tools   []llm.Tool // the browser's, from tools/list
	last    int        // the last JSON-RPC request id
}

// browserError is the browser refusing an MCP message, or answering one in a way the API cannot read. Unlike a runner
// error, it leaves the execution going.
type browserError string

func (e browserError) Error() string { return string(e) }

// openBrowser starts execution e's browser and opens its MCP session: initialize, notifications/initialized, and
// tools/list once for the tools its LLM checks get.
func (a *Automation) openBrowser(ctx context.Context, e *execution) (*browser, error) {
	if err := a.Runner.Browser(ctx, e.id); err != nil {
		return nil, fromRunner(err)
	}
	b := &browser{runner: a.Runner, id: e.id}
	var init struct{ ProtocolVersion string }
	err := b.send(ctx, "initialize", map[string]any{
		"protocolVersion": mcpVersion, "capabilities": map[string]any{},
		"clientInfo": map[string]string{"name": "gantt-stop-me", "version": "1"},
	}, &init)
	if err != nil {
		return nil, err
	}
	b.version = cmp.Or(init.ProtocolVersion, mcpVersion)
	if err := b.send(ctx, "notifications/initialized", nil, nil); err != nil {
		return nil, err
	}
	var list struct {
		Tools []struct {
			Name, Description string
			InputSchema       json.RawMessage
		}
	}
	if err := b.send(ctx, "tools/list", nil, &list); err != nil {
		return nil, err
	}
	for _, t := range list.Tools {
		if !slices.Contains(devtools, t.Name) {
			b.tools = append(b.tools, llm.Tool{Name: t.Name, Description: t.Description, Parameters: t.InputSchema})
		}
	}
	return b, nil
}

// devtools are the tools the browser's --caps=devtools adds, which no model gets: the API records the browser itself,
// and browser_annotate waits for a person to draw on the page, which would hang the check.
// shortcut: the list is the DevTools section of the README of mcr.microsoft.com/playwright/mcp:v0.0.83, the runner's
// browser image; compare it with the new README's whenever the image changes, or a new devtools tool reaches models.
var devtools = []string{
	"browser_annotate", "browser_hide_highlight", "browser_highlight", "browser_resume", "browser_start_recording",
	"browser_start_tracing", "browser_start_video", "browser_stop_recording", "browser_stop_tracing", "browser_stop_video",
	"browser_video_chapter", "browser_video_hide_actions", "browser_video_show_actions",
}

// content is a tools/call result: what the tool answered, as text and images.
type content struct {
	Content []struct{ Type, Text, Data, MimeType string }
	IsError bool
}

// call calls the browser's tool name with arguments, the model's JSON object, and returns its text for the model,
// cut to maxLog bytes.
func (b *browser) call(ctx context.Context, name, arguments string) (string, error) {
	if strings.TrimSpace(arguments) == "" {
		arguments = "{}"
	}
	if !json.Valid([]byte(arguments)) {
		return "", browserError(name + " needs its arguments as a JSON object")
	}
	var res content
	if err := b.send(ctx, "tools/call", map[string]any{"name": name, "arguments": json.RawMessage(arguments)}, &res); err != nil {
		return "", err
	}
	var parts []string
	for _, c := range res.Content {
		if c.Type == "text" {
			parts = append(parts, c.Text)
		} else {
			parts = append(parts, "("+c.Type+" not shown)")
		}
	}
	answer := strings.Join(parts, "\n")
	if res.IsError {
		answer = "Error: " + answer
	}
	return head(answer, maxLog), nil
}

// screenshot takes a PNG of the browser's page with browser_take_screenshot, or is nil when it gives none. A failure
// is not the check's: it reported already, and a runner that stopped stops the next request.
func (b *browser) screenshot(ctx context.Context) []byte {
	var res content
	if b.send(ctx, "tools/call", map[string]any{"name": "browser_take_screenshot", "arguments": map[string]string{"type": "png"}}, &res) != nil {
		return nil
	}
	for _, c := range res.Content {
		if png, err := base64.StdEncoding.DecodeString(c.Data); c.Type == "image" && c.MimeType == "image/png" && err == nil {
			return png
		}
	}
	return nil
}

// startVideo starts recording the browser's page as name, at 1280x720 and without the animated cursor, which slows
// every action.
func (b *browser) startVideo(ctx context.Context, name string) error {
	return b.record(ctx, "browser_start_video", map[string]any{
		"filename": runner.VideoFile(name), "size": map[string]int{"width": 1280, "height": 720}, "cursor": false,
	})
}

// stopVideo stops the recording startVideo started as name and returns it, as WebM, from the runner, which then
// deletes it.
func (b *browser) stopVideo(ctx context.Context, name string) ([]byte, error) {
	if err := b.record(ctx, "browser_stop_video", map[string]any{}); err != nil {
		return nil, err
	}
	return b.runner.Video(ctx, b.id, name)
}

// record calls the browser's recording tool name with arguments. The tool answering an error is a browserError.
func (b *browser) record(ctx context.Context, name string, arguments map[string]any) error {
	var res content
	if err := b.send(ctx, "tools/call", map[string]any{"name": name, "arguments": arguments}, &res); err != nil {
		return err
	}
	if res.IsError {
		var text []string
		for _, c := range res.Content {
			text = append(text, c.Text)
		}
		return browserError(name + " failed: " + strings.Join(text, "\n"))
	}
	return nil
}

// send sends one JSON-RPC message to the browser in b's session and, for a request, decodes its answer's result into
// out. A method under notifications/ is a notification, which gets no answer. The runner failing comes back through
// fromRunner; the runner or the browser refusing the message, or a JSON-RPC error, is a browserError.
func (b *browser) send(ctx context.Context, method string, params, out any) error {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
	}
	notification := strings.HasPrefix(method, "notifications/")
	if !notification {
		b.last++
		msg["id"] = b.last
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	answer, contentType, session, err := b.runner.MCP(ctx, b.id, b.session, b.version, body)
	// The browser's own refusal has no code; the runner's 404 not_found means it lost the execution.
	if re := (*runner.Error)(nil); errors.As(err, &re) && (re.Code == "" || re.Status != http.StatusNotFound) {
		return browserError(fmt.Sprintf("%s failed with HTTP %d: %s", method, re.Status, re.Message))
	}
	if err != nil {
		return fromRunner(err)
	}
	b.session = cmp.Or(b.session, session)
	if notification {
		return nil
	}
	return decodeRPC(method, answer, contentType, b.last, out)
}

// decodeRPC finds the JSON-RPC response with id in an MCP answer to method, which is the answer itself or one of its
// server-sent events, and decodes its result into out.
func decodeRPC(method string, answer []byte, contentType string, id int, out any) error {
	messages := []string{string(answer)}
	if strings.HasPrefix(contentType, "text/event-stream") {
		messages = nil
		for event := range strings.SplitSeq(strings.ReplaceAll(string(answer), "\r\n", "\n"), "\n\n") {
			var data []string
			for line := range strings.SplitSeq(event, "\n") {
				if d, ok := strings.CutPrefix(line, "data:"); ok {
					data = append(data, strings.TrimPrefix(d, " "))
				}
			}
			messages = append(messages, strings.Join(data, "\n"))
		}
	}
	for _, m := range messages {
		var msg struct {
			ID     *int
			Result json.RawMessage
			Error  *struct {
				Code    int
				Message string
			}
		}
		if json.Unmarshal([]byte(m), &msg) != nil || msg.ID == nil || *msg.ID != id {
			continue // a notification or request of the server's, or not JSON-RPC
		}
		if msg.Error != nil {
			return browserError(fmt.Sprintf("the browser answered %s with error %d: %s", method, msg.Error.Code, msg.Error.Message))
		}
		if json.Unmarshal(msg.Result, out) != nil {
			return browserError("the browser's answer to " + method + " is malformed")
		}
		return nil
	}
	return browserError("the browser's answer to " + method + " holds no response")
}
