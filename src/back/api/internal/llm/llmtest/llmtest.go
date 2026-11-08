// Package llmtest fakes an OpenAI-compatible LLM provider for tests: the model listing settings checks, the
// Decisions API and Chat Completions, answered from a script.
package llmtest

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/llm"
)

// The models a Fake lists unless a test changes Models.
const (
	DecisionModel = "test-decisions"
	ChatModel     = "test-chat"
)

// Fake is a provider on a test server. Register it in Settings with URL, Key and any of Models, as the base URL,
// API key and models. Every request needs the key, and decisions and chats need a listed model.
type Fake struct {
	Server *httptest.Server
	URL    string   // the base URL: Server.URL + "/v1"
	Key    string   // the only API key it accepts
	Models []string // what GET /models lists; change it before the API first calls the fake

	mux       *http.ServeMux
	mu        sync.Mutex // held while serving each request
	decide    func(DecisionsRequest) []llm.Answer
	replies   []Reply
	lastCall  int // the last tool call id given out
	decisions []DecisionsRequest
	chats     []ChatRequest
	failures  map[string]int // route pattern -> the status it answers instead
}

// DecisionsRequest is a POST /decisions the fake received.
type DecisionsRequest struct {
	Model     string
	Input     string
	Questions []llm.Question
}

// ChatRequest is a POST /chat/completions the fake received.
type ChatRequest struct {
	Model    string
	Messages []llm.Message // an assistant message's null content reads as ""
	Tools    []llm.Tool
}

// Reply is one scripted chat completion: the tool calls the model makes, in order, and the text it says, if any.
type Reply struct {
	Text  string
	Calls []llm.FunctionCall
}

// New starts a Fake, closed when the test ends.
func New(t testing.TB) *Fake {
	f := &Fake{
		Key:      "sk-llmtest-" + rand.Text(),
		Models:   []string{DecisionModel, ChatModel},
		mux:      http.NewServeMux(),
		failures: map[string]int{},
	}
	f.mux.HandleFunc("GET /models", f.serveModels)
	f.mux.HandleFunc("POST /decisions", f.serveDecisions)
	f.mux.HandleFunc("POST /chat/completions", f.serveChat)
	f.Server = httptest.NewServer(http.StripPrefix("/v1", http.HandlerFunc(f.serve)))
	f.URL = f.Server.URL + "/v1"
	t.Cleanup(f.Server.Close)
	return f
}

// Decide makes answer answer every decisions request from now on. It runs with the fake locked, so it must not call
// the fake. Until a test sets one, each question is answered with its first choice at probability 1.
func (f *Fake) Decide(answer func(DecisionsRequest) []llm.Answer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.decide = answer
}

// Reply queues the answers to the next chat completions, one each. Once the queue is empty, a chat answers 500.
func (f *Fake) Reply(replies ...Reply) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, replies...)
}

// Fail makes the route pattern, such as "POST /chat/completions", answer status instead of serving the request.
// Status 0 serves it again.
func (f *Fake) Fail(pattern string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures[pattern] = status
}

// Decisions returns every decisions request received so far, in order.
func (f *Fake) Decisions() []DecisionsRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.decisions)
}

// Chats returns every chat completions request received so far, in order. Each holds the whole conversation.
func (f *Fake) Chats() []ChatRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.chats)
}

// serve checks the key and, unless the route is made to fail, serves the request with mu held.
func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+f.Key {
		fail(w, http.StatusUnauthorized, "Incorrect API key provided.")
		return
	}
	if _, pattern := f.mux.Handler(r); f.failures[pattern] != 0 {
		fail(w, f.failures[pattern], "llmtest: injected failure")
		return
	}
	f.mux.ServeHTTP(w, r)
}

func (f *Fake) serveModels(w http.ResponseWriter, r *http.Request) {
	data := []map[string]string{}
	for _, m := range f.Models {
		data = append(data, map[string]string{"id": m, "object": "model"})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (f *Fake) serveDecisions(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Model     string `json:"model"`
		Input     string `json:"input"`
		Questions []struct {
			Type string `json:"type"`
			llm.Question
		} `json:"questions"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		fail(w, http.StatusBadRequest, "llmtest: the body is not JSON")
		return
	}
	req := DecisionsRequest{Model: in.Model, Input: in.Input}
	for _, q := range in.Questions {
		req.Questions = append(req.Questions, q.Question)
	}
	f.decisions = append(f.decisions, req)
	if !f.listed(w, in.Model) {
		return
	}
	for _, q := range in.Questions {
		if q.Type != "choice" {
			fail(w, http.StatusBadRequest, fmt.Sprintf("llmtest: question %q has type %q, not choice", q.Name, q.Type))
			return
		}
	}
	var answers []llm.Answer
	if f.decide != nil {
		answers = f.decide(req)
	} else {
		for _, q := range req.Questions {
			if len(q.Choices) > 0 {
				v := q.Choices[0].Value
				answers = append(answers, llm.Answer{Type: "choice", Name: q.Name, Choice: v,
					Probabilities: []llm.Probability{{Value: v, Probability: 1}}, Confidence: 1})
			}
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"answers": append([]llm.Answer{}, answers...)})
}

func (f *Fake) serveChat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Model    string        `json:"model"`
		Messages []llm.Message `json:"messages"`
		Tools    []struct {
			Type     string   `json:"type"`
			Function llm.Tool `json:"function"`
		} `json:"tools"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		fail(w, http.StatusBadRequest, "llmtest: the body is not JSON")
		return
	}
	req := ChatRequest{Model: in.Model, Messages: in.Messages}
	for _, t := range in.Tools {
		req.Tools = append(req.Tools, t.Function)
	}
	f.chats = append(f.chats, req)
	if !f.listed(w, in.Model) {
		return
	}
	for _, t := range in.Tools {
		if t.Type != "function" {
			fail(w, http.StatusBadRequest, fmt.Sprintf("llmtest: tool %q has type %q, not function", t.Function.Name, t.Type))
			return
		}
	}
	if msg := checkMessages(in.Messages); msg != "" {
		fail(w, http.StatusBadRequest, "llmtest: "+msg)
		return
	}
	if len(f.replies) == 0 {
		fail(w, http.StatusInternalServerError, "llmtest: no reply scripted")
		return
	}
	reply := f.replies[0]
	f.replies = f.replies[1:]
	message := map[string]any{"role": "assistant", "content": nil}
	if reply.Text != "" {
		message["content"] = reply.Text
	}
	finish := "stop"
	if len(reply.Calls) > 0 {
		var calls []llm.ToolCall
		for _, c := range reply.Calls {
			f.lastCall++
			calls = append(calls, llm.ToolCall{ID: fmt.Sprintf("call_%d", f.lastCall), Type: "function", Function: c})
		}
		message["tool_calls"], finish = calls, "tool_calls"
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id": fmt.Sprintf("chatcmpl-%d", len(f.chats)), "object": "chat.completion", "model": in.Model,
		"choices": []map[string]any{{"index": 0, "message": message, "finish_reason": finish}},
	})
}

// listed answers 404 like OpenAI, and returns false, when the fake does not list model.
func (f *Fake) listed(w http.ResponseWriter, model string) bool {
	if !slices.Contains(f.Models, model) {
		fail(w, http.StatusNotFound, fmt.Sprintf("The model `%s` does not exist.", model))
		return false
	}
	return true
}

// checkMessages says what is wrong with a conversation, as OpenAI would refuse it, or returns "": every assistant
// tool call must be answered by a tool message, before any other message, and every tool message must answer one.
func checkMessages(messages []llm.Message) string {
	if len(messages) == 0 {
		return "no messages"
	}
	open := map[string]bool{} // tool call ids not answered yet
	for i, m := range messages {
		if m.Role != "tool" && len(open) > 0 {
			return fmt.Sprintf("message %d comes before every tool call is answered", i)
		}
		switch m.Role {
		case "system", "user":
		case "assistant":
			for _, c := range m.ToolCalls {
				open[c.ID] = true
			}
		case "tool":
			if !open[m.ToolCallID] {
				return fmt.Sprintf("message %d answers no open tool call", i)
			}
			delete(open, m.ToolCallID)
		default:
			return fmt.Sprintf("message %d has role %q", i, m.Role)
		}
	}
	if len(open) > 0 {
		return "the last tool calls are not answered"
	}
	return ""
}

// fail answers status with an OpenAI-shaped error.
func fail(w http.ResponseWriter, status int, message string) {
	httpx.JSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}
