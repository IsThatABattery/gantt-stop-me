package planning_test

import (
	"net/http"
	"testing"
)

func TestApprove(t *testing.T) {
	f := setup(t)
	pl := f.toReview()
	briefing := f.start("Imports", "Import a CSV.")
	f.act(briefing.ID, "approve", nil).Expect(http.StatusConflict, "wrong_status")
	got := decode(t, f.act(pl.ID, "approve", nil), http.StatusOK)
	if got.Status != "approved" || orNull(got.Pending) != "generate" || got.Version != pl.Version+1 {
		t.Fatalf("approved plan %+v", got)
	}
	f.act(pl.ID, "approve", nil).Expect(http.StatusConflict, "wrong_status")

	// Generating tasks waits for the task agent: the worker leaves it.
	chats := len(f.llm.Chats())
	f.llm.Reply(reply(ask("Who imports?")))
	f.work()
	if got = f.plan(pl.ID); got.Status != "approved" || orNull(got.Pending) != "generate" || got.Version != pl.Version+1 {
		t.Fatalf("approved plan after work %+v", got)
	}
	if n := len(f.llm.Chats()); n != chats+1 {
		t.Fatalf("%d chat requests, want only the briefing plan's turn", n-chats)
	}
}
