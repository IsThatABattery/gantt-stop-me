package llm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/llm/llmtest"
)

// client is a client for the fake, through the guard as the API calls providers.
func client(f *llmtest.Fake) *llm.Client {
	return &llm.Client{BaseURL: f.URL, Key: f.Key, HTTP: llm.HTTPClient(true, llm.CallTimeout)}
}

var questions = []llm.Question{
	{Name: "test_3", Instructions: "Which workflow can run test 3?", Choices: []llm.Choice{
		{Value: "backend", Description: "Go API, endpoints, database"},
		{Value: "frontend", Description: "React UI seen in a browser, against the real API"},
	}},
	{Name: "test_4", Instructions: "Which workflow can run test 4?", Choices: []llm.Choice{
		{Value: "backend", Description: "Go API, endpoints, database"},
		{Value: "frontend", Description: "React UI seen in a browser, against the real API"},
	}},
}

func TestDecide(t *testing.T) {
	f := llmtest.New(t)
	backend := llm.Answer{Type: "choice", Name: "test_3", Choice: "backend", Confidence: 0.9,
		Probabilities: []llm.Probability{{Value: "backend", Probability: 0.92}, {Value: "frontend", Probability: 0.08}}}
	frontend := llm.Answer{Type: "choice", Name: "test_4", Choice: "frontend", Confidence: 0.7,
		Probabilities: []llm.Probability{{Value: "frontend", Probability: 0.75}, {Value: "backend", Probability: 0.25}}}
	f.Decide(func(llmtest.DecisionsRequest) []llm.Answer {
		return []llm.Answer{frontend, backend} // not in question order
	})

	answers, err := client(f).Decide(context.Background(), llmtest.DecisionModel, "Issue: Sign-up rejects empty emails", questions)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]llm.Answer{"test_3": backend, "test_4": frontend}; !reflect.DeepEqual(answers, want) {
		t.Fatalf("answers %+v, want %+v", answers, want)
	}
	// The fake refuses a question whose type is not choice, so this also checks the type was sent.
	want := []llmtest.DecisionsRequest{{Model: llmtest.DecisionModel, Input: "Issue: Sign-up rejects empty emails", Questions: questions}}
	if got := f.Decisions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("sent %+v, want %+v", got, want)
	}
}

func TestDecideLeavesOutRefusals(t *testing.T) {
	f := llmtest.New(t)
	f.Decide(func(llmtest.DecisionsRequest) []llm.Answer {
		return []llm.Answer{{Type: "refusal", Name: "test_3"}, {Type: "choice", Name: "test_4", Choice: "backend"}}
	})

	answers, err := client(f).Decide(context.Background(), llmtest.DecisionModel, "Issue", questions)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := answers["test_3"]; ok || len(answers) != 1 || answers["test_4"].Choice != "backend" {
		t.Fatalf("answers %+v, want only test_4's", answers)
	}
}

func TestDecideErrors(t *testing.T) {
	serve := func(h http.HandlerFunc) string {
		srv := httptest.NewServer(h)
		t.Cleanup(srv.Close)
		return srv.URL
	}
	f := llmtest.New(t)
	guarded := llm.HTTPClient(true, llm.CallTimeout)
	cases := map[string]struct {
		client *llm.Client
		model  string
	}{
		"wrong key":     {&llm.Client{BaseURL: f.URL, Key: "sk-wrong", HTTP: guarded}, llmtest.DecisionModel},
		"unknown model": {client(f), "gpt-unknown"},
		"blocked":       {&llm.Client{BaseURL: f.URL, Key: f.Key, HTTP: llm.HTTPClient(false, llm.CallTimeout)}, llmtest.DecisionModel},
		"error": {&llm.Client{HTTP: guarded, BaseURL: serve(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte(`{"answers":[],"error":{"message":"upstream secret detail"}}`))
		})}, ""},
		"not json": {&llm.Client{HTTP: guarded, BaseURL: serve(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("upstream secret detail"))
		})}, ""},
		"over 4 MiB": {&llm.Client{HTTP: guarded, BaseURL: serve(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"answers":[]` + strings.Repeat(" ", 4<<20) + `}`))
		})}, ""},
	}
	for name, c := range cases {
		answers, err := c.client.Decide(context.Background(), c.model, "Issue", questions)
		if err == nil || answers != nil || strings.Contains(err.Error(), "upstream secret detail") {
			t.Errorf("%s: %v, %v; want an error without the provider's answer", name, answers, err)
		}
	}
	if n := len(f.Decisions()); n != 1 { // only the unknown model got past the key check
		t.Fatalf("the fake served %d decisions", n)
	}
}

func TestUnreachableProviderErrorsNameNoAddress(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // nothing listens there now
	c := &llm.Client{BaseURL: srv.URL, HTTP: llm.HTTPClient(true, llm.CallTimeout)}
	_, err := c.Decide(context.Background(), llmtest.DecisionModel, "Issue", questions)
	if err == nil || err.Error() != "llm: POST /decisions could not reach the provider" {
		t.Fatalf("error = %v, want one without %s", err, srv.URL)
	}
}
