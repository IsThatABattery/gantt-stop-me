package planning_test

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestDraftEndsTheInterviewEarly(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.act(pl.ID, "draft", nil).Expect(http.StatusConflict, "agent_busy")
	f.llm.Reply(reply(ask("Who exports?")))
	f.work()

	pl = decode(t, f.act(pl.ID, "draft", nil), http.StatusOK)
	if orNull(pl.Pending) != "draft" || pl.Status != "briefing" {
		t.Fatalf("plan after draft %+v", pl)
	}
	f.act(pl.ID, "answer", map[string]string{"text": "Admins"}).Expect(http.StatusConflict, "agent_busy")
	f.llm.Reply(reply(update(sec("goal", "Export the board.", "covered"), sec("open_questions", []string{"Who exports?"}, "covered")), finish))
	f.work()

	pl = f.plan(pl.ID)
	if pl.Status != "needs_review" || pl.Pending != nil || pl.ExpectedQuestions != 1 ||
		!reflect.DeepEqual(pl.section("open_questions").Body, []any{"Who exports?"}) {
		t.Fatalf("plan after the draft turn %+v", pl)
	}
	if names := toolNames(f.llm.Chats()[1]); !slices.Equal(names, []string{"update_doc", "finish"}) {
		t.Fatalf("draft turn's tools %v", names)
	}
	if p := f.prompt(1); !strings.Contains(p, "The person asked for the draft now") || !strings.Contains(p, `"answer": null`) {
		t.Fatalf("draft prompt:\n%s", p)
	}
	f.act(pl.ID, "draft", nil).Expect(http.StatusConflict, "wrong_status")
}
