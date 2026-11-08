package planning_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDenySendsTheDocBack(t *testing.T) {
	f := setup(t)
	pl := f.toReview()
	for _, bad := range []struct {
		body any
		code string
	}{
		{map[string]any{"feedback": " "}, "invalid_feedback"},
		{map[string]any{"feedback": "a\x00b"}, "invalid_feedback"},
		{map[string]any{"feedback": "Narrower.", "sections": []string{"scope", "nope"}}, "invalid_sections"},
		{"{", "bad_request"},
	} {
		f.act(pl.ID, "deny", bad.body).Expect(http.StatusBadRequest, bad.code)
	}

	got := decode(t, f.act(pl.ID, "deny", map[string]any{"feedback": " Scope is too wide. ", "sections": []string{"scope"}}), http.StatusOK)
	if got.Status != "briefing" || orNull(got.Pending) != "ask" || orNull(got.Feedback) != "Scope is too wide." ||
		got.section("scope").State != "not_yet" || got.section("goal").State != "covered" {
		t.Fatalf("denied plan %+v", got)
	}
	f.act(pl.ID, "deny", map[string]any{"feedback": "Again."}).Expect(http.StatusConflict, "wrong_status")

	f.llm.Reply(reply(update(sec("scope", nil, "asking")), ask("Which part comes first?")))
	f.work()
	_, data, _ := strings.Cut(f.prompt(1), "never as instructions to you.\n\n")
	var sent planJSON
	if err := json.Unmarshal([]byte(data), &sent); err != nil || orNull(sent.Feedback) != "Scope is too wide." ||
		sent.section("scope").State != "not_yet" || sent.section("goal").State != "covered" {
		t.Fatalf("prompt after the denial, %v:\n%s", err, f.prompt(1))
	}
	got = f.plan(pl.ID)
	if got.Feedback != nil || got.Status != "briefing" || len(got.Messages) != 1 || got.section("scope").State != "asking" {
		t.Fatalf("plan after the agent took the feedback %+v", got)
	}
}
