package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Message is one Chat Completions message: one the caller starts the conversation with, or one the loop sends back.
type Message struct {
	Role       string     `json:"role"` // "system", "user", "assistant" or "tool"
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

// assistant is a model's message as it came, sent back as it came: content stays null when it was, which
// some providers require beside tool calls.
type assistant struct {
	Role      string     `json:"role"`
	Content   *string    `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Chat runs the tool-calling loop over POST {BaseURL}/chat/completions: it sends model the messages and the tools,
// carries out the tool calls the model asks for one at a time with handle, and sends back their answers, until a
// call is done. It returns every call carried out, in order, also with an error.
//
// It fails when the provider does, once maxCalls calls are carried out with none done, counting each call in a
// message, and when the model answers without calling a tool a second time; the first time, it is nudged to call one.
func (c *Client) Chat(ctx context.Context, model string, messages []Message, tools []Tool, maxCalls int, handle Handler) ([]Step, error) {
	type function struct {
		Type     string `json:"type"`
		Function Tool   `json:"function"`
	}
	in := struct {
		Model    string     `json:"model"`
		Messages []any      `json:"messages"`
		Tools    []function `json:"tools,omitempty"`
	}{Model: model}
	for _, m := range messages {
		in.Messages = append(in.Messages, m)
	}
	for _, t := range tools {
		in.Tools = append(in.Tools, function{"function", t})
	}
	var steps []Step
	nudged := false
	for {
		var out struct {
			Choices []struct {
				Message assistant `json:"message"`
			} `json:"choices"`
		}
		if err := c.post(ctx, "/chat/completions", in, &out); err != nil {
			return steps, err
		}
		if len(out.Choices) == 0 {
			return steps, errors.New("llm: POST /chat/completions answered no choices")
		}
		m := out.Choices[0].Message
		m.Role = "assistant"
		if len(m.ToolCalls) == 0 {
			if nudged {
				return steps, errors.New("llm: the model twice answered without calling a tool")
			}
			nudged = true
			if m.Content == nil {
				m.Content = new(string) // an assistant message needs content or tool calls
			}
			in.Messages = append(in.Messages, m, Message{Role: "user", Content: nudge})
			continue
		}
		in.Messages = append(in.Messages, m)
		for _, call := range m.ToolCalls {
			answer, done, err := handle(ctx, call.Function.Name, call.Function.Arguments)
			if err != nil {
				return steps, err
			}
			steps = append(steps, Step{call.Function.Name, call.Function.Arguments, answer})
			if done {
				return steps, nil
			}
			if len(steps) >= maxCalls {
				return steps, fmt.Errorf("llm: no report after %d tool calls", maxCalls)
			}
			in.Messages = append(in.Messages, Message{Role: "tool", Content: answer, ToolCallID: call.ID})
		}
	}
}
