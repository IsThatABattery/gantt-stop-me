package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Message is one message the caller starts the conversation with, sent as a Responses input message. ToolCalls and
// ToolCallID are only for llmtest, which reads the conversation back as messages.
type Message struct {
	Role       string     `json:"role"` // "system" or "user"; llmtest also reads "assistant" and "tool"
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // the assistant's
	ToolCallID string     `json:"tool_call_id,omitempty"` // the call a "tool" message answers
}

// ToolCall is one tool call on an assistant message.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall is the function a ToolCall calls.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON text, as the model wrote it, so it may not be valid
}

// Tool is a function the chat model may call.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"` // a JSON Schema object, such as an MCP tool's inputSchema
}

// Step is one tool call the loop carried out, as the caller stores it in a transcript.
type Step struct {
	Tool      string `json:"tool"`
	Arguments string `json:"arguments"`
	Answer    string `json:"answer"`
}

// Handler carries out one tool call with the model's arguments and returns the answer sent back to the model.
// done ends the loop after this call, as report does. An error ends the loop with that error.
type Handler func(ctx context.Context, name, arguments string) (answer string, done bool, err error)

// nudge is sent once when the model answers without calling a tool, since only a call that is done ends the loop.
// It names no tool, since each caller's tools differ.
const nudge = "Answer by calling one of the tools."

// outputItem is the part of a Responses output item the loop reads. The item itself goes back to the model as it
// came, so reasoning and messages keep every field.
type outputItem struct {
	Type      string `json:"type"` // "function_call", "message", "reasoning", ...
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Chat runs the tool-calling loop over POST {BaseURL}/responses: it sends model the messages and the tools,
// carries out the function calls the model asks for one at a time with handle, and sends back their outputs, until
// a call is done. It returns every call carried out, in order, also with an error.
//
// Nothing is stored at the provider: each request sends the whole conversation, with reasoning as encrypted content.
//
// It fails when the provider does, once maxCalls calls are carried out with none done, counting each call in an
// answer, and when the model answers without calling a tool a second time; the first time, it is nudged to call one.
func (c *Client) Chat(ctx context.Context, model string, messages []Message, tools []Tool, maxCalls int, handle Handler) ([]Step, error) {
	type function struct {
		Type string `json:"type"`
		// false: strict, the Responses API's default, makes every property required, so the model can leave none out.
		Strict bool `json:"strict"`
		Tool
	}
	in := struct {
		Model   string     `json:"model"`
		Input   []any      `json:"input"`
		Tools   []function `json:"tools,omitempty"`
		Store   bool       `json:"store"`
		Include []string   `json:"include"`
	}{Model: model, Include: []string{"reasoning.encrypted_content"}}
	for _, m := range messages {
		in.Input = append(in.Input, m)
	}
	for _, t := range tools {
		in.Tools = append(in.Tools, function{Type: "function", Tool: t})
	}
	var steps []Step
	nudged := false
	for {
		var out struct {
			Output []json.RawMessage `json:"output"`
		}
		if err := c.post(ctx, "/responses", in, &out); err != nil {
			return steps, err
		}
		var calls []outputItem
		for _, raw := range out.Output {
			var item outputItem
			if json.Unmarshal(raw, &item) != nil {
				return steps, errors.New("llm: POST /responses answered a malformed output item")
			}
			in.Input = append(in.Input, raw)
			if item.Type == "function_call" {
				calls = append(calls, item)
			}
		}
		if len(calls) == 0 {
			if nudged {
				return steps, errors.New("llm: the model twice answered without calling a tool")
			}
			nudged = true
			in.Input = append(in.Input, Message{Role: "user", Content: nudge})
			continue
		}
		for _, call := range calls {
			answer, done, err := handle(ctx, call.Name, call.Arguments)
			if err != nil {
				return steps, err
			}
			steps = append(steps, Step{call.Name, call.Arguments, answer})
			if done {
				return steps, nil
			}
			if len(steps) >= maxCalls {
				return steps, fmt.Errorf("llm: no report after %d tool calls", maxCalls)
			}
			in.Input = append(in.Input, map[string]string{"type": "function_call_output", "call_id": call.CallID, "output": answer})
		}
	}
}
