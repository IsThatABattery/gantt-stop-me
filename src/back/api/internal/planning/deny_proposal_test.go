package planning_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestDenyProposal(t *testing.T) {
	f := setup(t)
	pl := f.generate(f.approved(),
		task("Export endpoint", nil, script("go test ./...")),
		task("Export button", []int{1}, script("npm test")),
		task("Export docs", nil, script("make docs")))
	pl = decode(t, f.editProposal(pl, 2, map[string]any{"title": "CSV button", "version": pl.Version}), http.StatusOK)

	for _, body := range []map[string]string{{"feedback": " ", "outcome": "revise"}, {"feedback": "Later.", "outcome": "later"}} {
		code := "invalid_feedback"
		if body["outcome"] == "later" {
			code = "invalid_outcome"
		}
		f.review(f.ada, pl, 2, "deny", body).Expect(http.StatusBadRequest, code)
	}
	const feedback = "Use a menu item, not a button."
	got := decode(t, f.review(f.ada, pl, 2, "deny", map[string]string{"feedback": " " + feedback, "outcome": "revise"}), http.StatusOK)
	if p := got.Proposals[1]; p.Status != "sent_back" || orNull(p.Feedback) != feedback || orNull(got.Pending) != "revise" {
		t.Fatalf("plan after revise %+v", got)
	}
	f.review(f.ada, got, 1, "approve", nil).Expect(http.StatusConflict, "agent_busy")

	// A failed revision is retried as one.
	f.work() // the fake answers 500 with no reply scripted
	if got = f.plan(pl.ID); got.Pending != nil || got.Error == nil || got.Proposals[1].Status != "sent_back" {
		t.Fatalf("plan after a failed revision %+v", got)
	}
	if got = decode(t, f.act(pl.ID, "retry", nil), http.StatusOK); orNull(got.Pending) != "revise" {
		t.Fatalf("plan after retry %+v", got)
	}
	f.llm.Reply(reply(task("Export menu item", []int{1}, script("npm test"))))
	f.work()
	got = f.plan(pl.ID)
	if p := got.Proposals[1]; got.Pending != nil || got.Error != nil || p.ID != pl.Proposals[1].ID || p.Position != 2 ||
		p.Status != "needs_review" || p.Title != "Export menu item" || p.Feedback != nil || len(got.Proposals) != 3 {
		t.Fatalf("plan after the revision %+v", got)
	}
	chats := f.llm.Chats()
	if chat := chats[len(chats)-1]; !slices.Equal(toolNames(chat), []string{"propose_task"}) {
		t.Fatalf("revision tools %v", toolNames(chat))
	}
	for _, want := range []string{"sent back task 2", "keeps number 2", `"feedback": "` + feedback + `"`, `"title": "CSV button"`} {
		if prompt := chats[len(chats)-1].Messages[0].Content; !strings.Contains(prompt, want) {
			t.Errorf("revision prompt lacks %q:\n%s", want, prompt)
		}
	}

	// A dropped proposal keeps its feedback, and those that needed it no longer name it.
	got = decode(t, f.review(f.ada, got, 1, "deny", map[string]string{"feedback": "Not needed.", "outcome": "drop"}), http.StatusOK)
	if p := got.Proposals[0]; p.Status != "dropped" || orNull(p.Feedback) != "Not needed." || got.Status != "approved" {
		t.Fatalf("plan after drop %+v", got)
	}
	f.review(f.ada, got, 1, "deny", map[string]string{"feedback": "Again.", "outcome": "drop"}).Expect(http.StatusConflict, "wrong_status")
	// The agent's rewrite is its own: approving it is not an edit.
	if got = decode(t, f.review(f.ada, got, 2, "approve", nil), http.StatusOK); got.Proposals[1].Status != "approved" {
		t.Fatalf("proposal %+v, want it approved", got.Proposals[1])
	}
	f.expectTask(got.Proposals[1], 1, "ada", "Build Export menu item.\n\n## Acceptance criteria\n- [ ] Export menu item works\n\n"+
		"## Tests\n- [ ] `npm test`\n")
	if got = decode(t, f.review(f.ada, got, 3, "approve", nil), http.StatusOK); got.Status != "done" {
		t.Fatalf("plan %+v, want it done", got)
	}
	if n := f.tasks(); n != 2 {
		t.Fatalf("%d tasks, want the two approved", n)
	}
}

// Dropping every proposal leaves nothing to wait for.
func TestDropEveryProposal(t *testing.T) {
	f := setup(t)
	pl := f.generate(f.approved(), task("Export", nil, script("make")))
	if got := decode(t, f.review(f.ada, pl, 1, "deny", map[string]string{"feedback": "Out of scope.", "outcome": "drop"}), http.StatusOK); got.Status != "done" {
		t.Fatalf("plan %+v, want it done", got)
	}
}
