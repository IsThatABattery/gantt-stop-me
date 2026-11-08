package automation_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/runner/runnertest"
)

// shownRun is a test run as GET run answers it.
type shownRun struct {
	listedRun
	Executions []struct {
		ID                    int64
		Workflow, Status      string
		Attempts              int
		LogTail               string
		StartedAt, FinishedAt *time.Time
	}
	Results []shownResult
}

// shownResult is a test result as GET run answers it.
type shownResult struct {
	ID            int64
	Position      int
	Text, Kind    string
	Pinned        bool
	ExecutionID   *int64
	Workflow      *string
	Probabilities map[string]float64
	Status        string
	Detail        string
	Transcript    json.RawMessage
	HasScreenshot bool
	HasVideo      bool
}

func TestGetRun(t *testing.T) {
	f := withLLM(t)
	f.tests(1, "## Tests\n- [ ] `go test ./...`\n- [ ] [api] `make`\n- [ ] Signing up with an empty email answers 400\n")
	f.Runner.Script(0, runnertest.Script{Log: "built", Commands: map[string]runnertest.Exit{"go test ./...": {Output: "ok"}}})
	f.llm.Reply(call("run_command", `{"command": "true"}`), reported("pass", "It refused."))
	n := f.ready()
	f.work()
	f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "") // the run stays readable once it is off

	reader, _, _ := f.member("bob", "pull")
	res := f.Do("GET", f.path(fmt.Sprintf("/runs/%d", f.lastRun())), nil, reader)
	res.Expect(http.StatusOK, "")
	var got shownRun
	res.JSON(&got)
	if got.Status != "error" || *got.PRNumber != n || *got.HeadSHA != "a1" || got.Reason != nil || got.FinishedAt == nil {
		t.Fatalf("run = %+v", got.listedRun)
	}
	if len(got.Executions) != 1 {
		t.Fatalf("executions = %+v, want one", got.Executions)
	}
	x := got.Executions[0]
	if x.Workflow != "default" || x.Status != "passed" || x.Attempts != 1 || x.LogTail != "built\n$ go test ./...\nok\n" ||
		x.StartedAt == nil || x.FinishedAt == nil {
		t.Fatalf("execution = %+v", x)
	}
	workflow := "default"
	want := []shownResult{
		{Position: 1, Text: "`go test ./...`", Kind: "script", ExecutionID: &x.ID, Workflow: &workflow,
			Probabilities: map[string]float64{}, Status: "passed", Detail: "Exit code 0.\nok", Transcript: json.RawMessage("null")},
		{Position: 2, Text: "`make`", Kind: "script", Pinned: true, Probabilities: map[string]float64{},
			Status: "error", Detail: "Test 2 is pinned to [api], which names no workflow.", Transcript: json.RawMessage("null")},
		{Position: 3, Text: "Signing up with an empty email answers 400", Kind: "llm", ExecutionID: &x.ID, Workflow: &workflow,
			Probabilities: map[string]float64{}, Status: "passed", Detail: "It refused."},
	}
	for i := range got.Results {
		if got.Results[i].ID == 0 {
			t.Fatalf("result %d has no id", i)
		}
		got.Results[i].ID = 0
	}
	var steps []llm.Step
	if len(got.Results) == 3 {
		json.Unmarshal(got.Results[2].Transcript, &steps)
		got.Results[2].Transcript = nil
	}
	if !reflect.DeepEqual(got.Results, want) {
		t.Fatalf("results = %+v, want %+v", got.Results, want)
	}
	if len(steps) != 2 || steps[0].Tool != "run_command" || steps[0].Answer != "Exit code 0.\n" || steps[1].Tool != "report" {
		t.Fatalf("transcript = %+v", steps)
	}
}

func TestGetRunNeedsAccess(t *testing.T) {
	f := withLLM(t)
	f.tests(1, twoTests)
	f.ready()
	run := f.path(fmt.Sprintf("/runs/%d", f.lastRun()))
	f.Do("GET", f.path("/runs/999"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	stranger := f.SignUp("eve@example.com", "correct horse")
	f.LinkGitHub(stranger, "eve")
	f.Do("GET", run, nil, stranger).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", run, nil).Expect(http.StatusUnauthorized, "unauthorized")
}
