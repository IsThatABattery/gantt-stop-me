package planning_test

import (
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/llm/llmtest"
)

func TestReviewOffApprovesTheDoc(t *testing.T) {
	f := setup(t)
	f.Do("PUT", "/api/me/planning", map[string]any{"providerId": f.provider, "model": llmtest.ChatModel, "review": false}, f.ada).
		Expect(http.StatusOK, "")
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.llm.Reply(reply(ask("Who exports?")))
	f.work()
	f.act(pl.ID, "draft", nil).Expect(http.StatusOK, "")
	f.llm.Reply(reply(update(sec("goal", "Export the board.", "covered")), finish))
	f.work()
	if pl = f.plan(pl.ID); pl.Status != "approved" || orNull(pl.Pending) != "generate" || pl.section("goal").State != "covered" {
		t.Fatalf("plan %+v, want it approved without review", pl)
	}
}

func TestWorkTakesEveryPlanOnce(t *testing.T) {
	f := setup(t)
	a := f.start("Exports", "Export a CSV.")
	b := f.start("Imports", "Import a CSV.")
	f.llm.Reply(reply(ask("Export what?")), reply(ask("Import what?")), reply(ask("Never asked")))
	f.work()
	if a, b = f.plan(a.ID), f.plan(b.ID); len(a.Messages) != 1 || a.Messages[0].Text != "Export what?" ||
		len(b.Messages) != 1 || b.Messages[0].Text != "Import what?" {
		t.Fatalf("plans after one pass %+v, %+v", a, b)
	}
	if n := len(f.llm.Chats()); n != 2 {
		t.Fatalf("%d chat requests in one pass, want one turn per plan", n)
	}
	f.work()
	if n := len(f.llm.Chats()); n != 2 {
		t.Fatalf("%d chat requests after a pass with nothing pending", n)
	}
}
