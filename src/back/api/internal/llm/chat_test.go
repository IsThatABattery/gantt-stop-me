package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/llm/llmtest"
)

var tools = []llm.Tool{
	{Name: "run_command", Description: "Run a shell command.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`)},
	{Name: "report", Description: "Report the outcome.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"outcome":{"type":"string"},"reason":{"type":"string"}}}`)},
}

var first = []llm.Message{
	{Role: "system", Content: "Verify with the tools, then call report once."},
	{Role: "user", Content: "Check: signing up with an empty email answers 400."},
}

// handled records every call handle carries out.
type handled struct{ calls []string }

// handle answers run_command with "ran" and its arguments, and ends the loop at report.
func (h *handled) handle(_ context.Context, name, arguments string) (string, bool, error) {
	h.calls = append(h.calls, name)
	if name == "report" {
		return "", true, nil
	}
	return "ran " + arguments, false, nil
}

func run(command string) llm.FunctionCall {
	return llm.FunctionCall{Name: "run_command", Arguments: `{"command":"` + command + `"}`}
}

var report = llm.FunctionCall{Name: "report", Arguments: `{"outcome":"pass","reason":"400 as expected"}`}

// chat runs the loop against f with first, tools and a cap of 50.
func chat(f *llmtest.Fake, h *handled) ([]llm.Step, error) {
	return client(f).Chat(context.Background(), llmtest.ChatModel, first, tools, 50, h.handle)
}

func TestChat(t *testing.T) {
	f := llmtest.New(t)
	f.Reply(llmtest.Reply{Calls: []llm.FunctionCall{run("go test")}}, llmtest.Reply{Calls: []llm.FunctionCall{report}})

	steps, err := chat(f, &handled{})
	if err != nil {
		t.Fatal(err)
	}
	want := []llm.Step{
		{Tool: "run_command", Arguments: `{"command":"go test"}`, Answer: `ran {"command":"go test"}`},
		{Tool: "report", Arguments: report.Arguments},
	}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps %+v, want %+v", steps, want)
	}
	chats := f.Chats()
	if len(chats) != 2 {
		t.Fatalf("%d chat requests, want 2", len(chats))
	}
	if c := chats[0]; c.Model != llmtest.ChatModel || !reflect.DeepEqual(c.Messages, first) || !reflect.DeepEqual(c.Tools, tools) {
		t.Fatalf("first request %+v", c)
	}
	sent := append(first[:len(first):len(first)],
		llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call_1", Type: "function", Function: run("go test")}}},
		llm.Message{Role: "tool", Content: `ran {"command":"go test"}`, ToolCallID: "call_1"},
	)
	if got := chats[1].Messages; !reflect.DeepEqual(got, sent) {
		t.Fatalf("second request's messages %+v, want %+v", got, sent)
	}
}

func TestChatSeveralCallsInOneMessage(t *testing.T) {
	f := llmtest.New(t)
	f.Reply(llmtest.Reply{Text: "Running both.", Calls: []llm.FunctionCall{run("a"), run("b")}}, llmtest.Reply{Calls: []llm.FunctionCall{report}})

	h := &handled{}
	steps, err := chat(f, h)
	if err != nil || len(steps) != 3 || steps[0].Arguments != `{"command":"a"}` || steps[1].Arguments != `{"command":"b"}` {
		t.Fatalf("steps %+v, %v", steps, err)
	}
	got := f.Chats()[1].Messages[len(first):]
	want := []llm.Message{
		{Role: "assistant", Content: "Running both.", ToolCalls: []llm.ToolCall{
			{ID: "call_1", Type: "function", Function: run("a")}, {ID: "call_2", Type: "function", Function: run("b")}}},
		{Role: "tool", Content: `ran {"command":"a"}`, ToolCallID: "call_1"},
		{Role: "tool", Content: `ran {"command":"b"}`, ToolCallID: "call_2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sent %+v, want %+v", got, want)
	}
}

func TestChatCapsToolCalls(t *testing.T) {
	three := llmtest.Reply{Calls: []llm.FunctionCall{run("a"), run("b"), run("c")}}

	// 17 messages of three calls ask for 51: the 50th, mid-message, ends the loop.
	f := llmtest.New(t)
	for range 17 {
		f.Reply(three)
	}
	h := &handled{}
	steps, err := chat(f, h)
	if err == nil || len(steps) != 50 || len(h.calls) != 50 || len(f.Chats()) != 17 {
		t.Fatalf("%d steps, %d calls, %d chat requests, %v; want an error after 50 calls", len(steps), len(h.calls), len(f.Chats()), err)
	}

	// A report as the 50th call still counts.
	f = llmtest.New(t)
	for range 16 {
		f.Reply(three)
	}
	f.Reply(llmtest.Reply{Calls: []llm.FunctionCall{run("a"), report}})
	if steps, err := chat(f, &handled{}); err != nil || len(steps) != 50 || steps[49].Tool != "report" {
		t.Fatalf("%d steps, %v; want the report as the 50th", len(steps), err)
	}
}

func TestChatEndsAtReport(t *testing.T) {
	f := llmtest.New(t)
	f.Reply(llmtest.Reply{Calls: []llm.FunctionCall{run("a"), report, run("b")}})

	h := &handled{}
	steps, err := chat(f, h)
	if err != nil || len(steps) != 2 || steps[1].Tool != "report" || !reflect.DeepEqual(h.calls, []string{"run_command", "report"}) {
		t.Fatalf("steps %+v, calls %v, %v; want the loop to stop at report", steps, h.calls, err)
	}
	if n := len(f.Chats()); n != 1 {
		t.Fatalf("%d chat requests after the report, want 1", n)
	}
}

func TestChatNudgesOnce(t *testing.T) {
	f := llmtest.New(t)
	f.Reply(llmtest.Reply{Text: "It answers 400."}, llmtest.Reply{Calls: []llm.FunctionCall{report}})

	steps, err := chat(f, &handled{})
	if err != nil || len(steps) != 1 || steps[0].Tool != "report" {
		t.Fatalf("steps %+v, %v", steps, err)
	}
	got := f.Chats()[1].Messages[len(first):]
	want := []llm.Message{{Role: "assistant", Content: "It answers 400."}, {Role: "user", Content: "Answer by calling one of the tools."}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sent %+v, want %+v", got, want)
	}

	// An empty answer is nudged too, and a second answer without a call, even after calls in between, ends the loop.
	f = llmtest.New(t)
	f.Reply(llmtest.Reply{}, llmtest.Reply{Calls: []llm.FunctionCall{run("a")}}, llmtest.Reply{Text: "Done."})
	steps, err = chat(f, &handled{})
	if err == nil || len(steps) != 1 || len(f.Chats()) != 3 {
		t.Fatalf("%d steps, %d chat requests, %v; want an error at the second answer without a call", len(steps), len(f.Chats()), err)
	}
}

func TestChatProviderFailure(t *testing.T) {
	f := llmtest.New(t)
	f.Fail("POST /chat/completions", 500)
	if steps, err := chat(f, &handled{}); err == nil || steps != nil || strings.Contains(err.Error(), "injected failure") {
		t.Fatalf("steps %+v, %v; want an error without the provider's answer", steps, err)
	}

	// Mid-loop: the fake answers 500 once its replies run out.
	f = llmtest.New(t)
	f.Reply(llmtest.Reply{Calls: []llm.FunctionCall{run("a")}})
	if steps, err := chat(f, &handled{}); err == nil || len(steps) != 1 {
		t.Fatalf("steps %+v, %v; want the call made before the failure, and an error", steps, err)
	}
}

func TestChatHandlerError(t *testing.T) {
	f := llmtest.New(t)
	f.Reply(llmtest.Reply{Calls: []llm.FunctionCall{run("a"), run("b")}})
	broken := errors.New("runner unreachable")

	calls := 0
	steps, err := client(f).Chat(context.Background(), llmtest.ChatModel, first, tools, 50,
		func(context.Context, string, string) (string, bool, error) {
			calls++
			return "", false, broken
		})
	if !errors.Is(err, broken) || len(steps) != 0 || calls != 1 {
		t.Fatalf("steps %+v, %d calls, %v; want the handler's error at once", steps, calls, err)
	}
}
