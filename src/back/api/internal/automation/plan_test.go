package automation_test

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/llm/llmtest"
)

// probabilities returns the stored probabilities of the project's test results, by run and position.
func (f *fixture) probabilities() []map[string]float64 {
	f.t.Helper()
	rows, err := f.DB.Query(context.Background(), `select probabilities from test_results order by run_id, position`)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var all []map[string]float64
	for rows.Next() {
		var p map[string]float64
		if err := rows.Scan(&p); err != nil {
			f.t.Fatal(err)
		}
		all = append(all, p)
	}
	return all
}

func TestRoutingWithOneWorkflow(t *testing.T) {
	for _, c := range []struct {
		name, file, workflow string
	}{
		{"no workflows file", "", "default"},
		{"one workflow", "workflows:\n  backend:\n    description: Go API\n", "backend"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := withLLM(t)
			f.tests(1, "## Tests\n- [ ] `go test ./...`\n- [ ] ["+c.workflow+"] `go vet ./...`\n")
			if c.file != "" {
				f.repo.AddFile("a1", ".gantt/workflows.yml", c.file)
			}
			n := f.ready()
			f.work()
			f.expectRuns(pass(n))
			f.expectResults(resultRow{1, "passed", "Exit code 0.", c.workflow, false}, resultRow{2, "passed", "Exit code 0.", c.workflow, true})
			if got := f.llm.Decisions(); len(got) != 0 {
				t.Fatalf("decisions requests = %+v, want none", got)
			}
		})
	}
}

func TestRoutingPinnedTestsNeedNoDecision(t *testing.T) {
	f := withLLM(t)
	f.tests(1, "## Tests\n- [ ] [frontend] `npm test`\n- [ ] [backend] `go test ./...`\n")
	f.repo.AddFile("a1", ".gantt/workflows.yml", twoWorkflows)
	n := f.ready()
	f.work()
	f.work() // one execution at a time
	f.expectRuns(pass(n))
	f.expectExecutions(executionRow{"backend", "passed", 1}, executionRow{"frontend", "passed", 1})
	f.expectResults(resultRow{1, "passed", "Exit code 0.", "frontend", true}, resultRow{2, "passed", "Exit code 0.", "backend", true})
	if got := f.llm.Decisions(); len(got) != 0 {
		t.Fatalf("decisions requests = %+v, want none", got)
	}
}

func TestRoutingAsksTheDecisionModel(t *testing.T) {
	f := withLLM(t)
	f.tests(1, "## Tests\n"+
		"- [ ] [backend] `go vet ./...`\n"+
		"- [ ] `go test ./...`\n"+
		"- [ ] Signing up with an empty email answers 400\n"+
		"  1. POST /api/auth/signup with no email.\n"+
		"- [ ] The board looks right on a phone\n"+
		"- [ ] Dragging a card closes its issue\n")
	f.repo.AddFile("a1", ".gantt/workflows.yml",
		"workflows:\n  backend:\n    description: Go API, endpoints, database\n  frontend:\n    description: React UI seen in a browser\n    port: 80\n")
	f.llm.Decide(func(r llmtest.DecisionsRequest) []llm.Answer {
		choice := func(name string, probs ...llm.Probability) llm.Answer {
			return llm.Answer{Type: "choice", Name: name, Choice: probs[0].Value, Probabilities: probs, Confidence: 0.9}
		}
		return []llm.Answer{
			choice("test_2", llm.Probability{Value: "backend", Probability: 0.92}, llm.Probability{Value: "frontend", Probability: 0.08}),
			choice("test_3", llm.Probability{Value: "backend", Probability: 0.3}, llm.Probability{Value: "frontend", Probability: 0.7}),
			{Type: "refusal", Name: "test_4"},
			// test_5 has no answer
		}
	})
	n := f.ready()
	f.work()
	f.work()

	got := f.llm.Decisions()
	choices := []llm.Choice{{Value: "backend", Description: "Go API, endpoints, database"}, {Value: "frontend", Description: "React UI seen in a browser"}}
	want := []llmtest.DecisionsRequest{{
		Model: llmtest.DecisionModel,
		Input: "Issue: One\n\n" +
			"Workflows:\n- backend: Go API, endpoints, database\n- frontend: React UI seen in a browser\n\n" +
			"Tests:\n" +
			"2. `go test ./...`\n" +
			"3. Signing up with an empty email answers 400\n   1. POST /api/auth/signup with no email.\n" +
			"4. The board looks right on a phone\n" +
			"5. Dragging a card closes its issue",
		Questions: []llm.Question{
			{Name: "test_2", Instructions: "Which workflow can run test 2?", Choices: choices},
			{Name: "test_3", Instructions: "Which workflow can run test 3?", Choices: choices},
			{Name: "test_4", Instructions: "Which workflow can run test 4?", Choices: choices},
			{Name: "test_5", Instructions: "Which workflow can run test 5?", Choices: choices},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.MarshalIndent(got, "", " ")
		t.Fatalf("decisions requests = %s", gotJSON)
	}

	f.expectExecutions(executionRow{"backend", "passed", 1}, executionRow{"frontend", "error", 1})
	f.expectResults(
		resultRow{1, "passed", "Exit code 0.", "backend", true},
		resultRow{2, "passed", "Exit code 0.", "backend", false},
		resultRow{3, "error", noChat, "frontend", false}, // no chat reply is scripted
		resultRow{4, "error", "Test 4 matches no workflow well enough to run. Best guesses: none. Pin it to one with [name].", "", false},
		resultRow{5, "error", "Test 5 matches no workflow well enough to run. Best guesses: none. Pin it to one with [name].", "", false},
	)
	if got, want := f.probabilities(), []map[string]float64{
		{}, {"backend": 0.92, "frontend": 0.08}, {"backend": 0.3, "frontend": 0.7}, {}, {},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("probabilities = %v, want %v", got, want)
	}
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "error", Finished: true})
	// The first error is test 3's, by position.
	if got := f.task(1); got.NeedsAttention != noChat {
		t.Fatalf("needs_attention = %q", got.NeedsAttention)
	}
	var specs []string
	for _, r := range f.Runner.Requests() {
		if r.Endpoint == "prepare" {
			specs = append(specs, r.Spec.Workflow)
		}
	}
	if !slices.Equal(specs, []string{"backend", "frontend"}) {
		t.Fatalf("prepared workflows = %v", specs)
	}
}
