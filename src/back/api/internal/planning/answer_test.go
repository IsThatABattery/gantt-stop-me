package planning_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestAnswerWaitsForTheQuestion(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.act(pl.ID, "answer", map[string]string{"text": "Too soon"}).Expect(http.StatusConflict, "agent_busy")

	f.llm.Reply(reply(ask("Who exports?")))
	f.work()
	for _, text := range []string{"", "  ", "a\x00b", strings.Repeat("é", 10001)} {
		f.act(pl.ID, "answer", map[string]string{"text": text}).Expect(http.StatusBadRequest, "invalid_text")
	}
	f.act(pl.ID, "answer", "{").Expect(http.StatusBadRequest, "bad_request")
	f.act(pl.ID, "answer", map[string]string{"text": "Admins"}).Expect(http.StatusOK, "")
	f.act(pl.ID, "answer", map[string]string{"text": "Twice"}).Expect(http.StatusConflict, "agent_busy")
	if pl = f.plan(pl.ID); len(pl.Messages) != 2 || pl.Answered != 1 {
		t.Fatalf("plan %+v, want one answer", pl)
	}

	f.Do("POST", f.path("/plans/999/answer"), map[string]string{"text": "Lost"}, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("POST", f.path("/plans/nope/answer"), map[string]string{"text": "Lost"}, f.ada).Expect(http.StatusNotFound, "not_found")
}

func TestAnswerNeedsAQuestion(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.work() // no reply scripted: the turn fails
	f.act(pl.ID, "answer", map[string]string{"text": "Admins"}).Expect(http.StatusConflict, "no_question")
	f.act(pl.ID, "answer", map[string]bool{"skip": true}).Expect(http.StatusConflict, "no_question")

	pl = f.toReview()
	f.act(pl.ID, "answer", map[string]string{"text": "Admins"}).Expect(http.StatusConflict, "wrong_status")
}
