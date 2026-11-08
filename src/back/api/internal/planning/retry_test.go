package planning_test

import (
	"net/http"
	"testing"
)

func TestRetryAFailedTurn(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.act(pl.ID, "retry", nil).Expect(http.StatusConflict, "agent_busy")
	f.llm.Reply(reply(update(sec("goal", "Lost.", "covered"))))
	f.work() // the fake answers 500 once its replies run out

	pl = f.plan(pl.ID)
	if pl.Status != "briefing" || pl.Pending != nil || orNull(pl.Error) != "The agent's turn failed: POST /responses answered HTTP 500." ||
		pl.section("goal").State != "not_yet" || len(pl.Messages) != 0 {
		t.Fatalf("plan after a failed turn %+v", pl)
	}
	got := decode(t, f.act(pl.ID, "retry", nil), http.StatusOK)
	if orNull(got.Pending) != "ask" || got.Error == nil {
		t.Fatalf("plan after retry %+v, want the job back and the error kept until a turn succeeds", got)
	}
	f.llm.Reply(reply(ask("Who exports?")))
	f.work()
	if got = f.plan(pl.ID); got.Error != nil || len(got.Messages) != 1 {
		t.Fatalf("plan after the retried turn %+v", got)
	}
	f.act(pl.ID, "retry", nil).Expect(http.StatusConflict, "nothing_to_retry")

	// A failed draft is retried as the draft.
	f.act(pl.ID, "draft", nil).Expect(http.StatusOK, "")
	f.work()
	if got = decode(t, f.act(pl.ID, "retry", nil), http.StatusOK); orNull(got.Pending) != "draft" {
		t.Fatalf("plan after retrying the draft %+v", got)
	}

	f.llm.Reply(reply(finish))
	f.work()
	f.act(pl.ID, "retry", nil).Expect(http.StatusConflict, "wrong_status")
}

func TestTurnWithoutAPlanningModel(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.Do("PUT", "/api/me/planning", agents(nil, nil, true), f.ada).Expect(http.StatusOK, "")
	f.work()
	if pl = f.plan(pl.ID); pl.Pending != nil || orNull(pl.Error) != "The plan's creator has no interviewer model. They pick one in Settings, then someone retries." {
		t.Fatalf("plan %+v", pl)
	}
	if n := len(f.llm.Chats()); n != 0 {
		t.Fatalf("%d chat requests without a planning model", n)
	}

	// The creator's settings, not the retrier's: bob's model does not run ada's plan.
	bob := f.member("bob", "push")
	f.Do("PUT", "/api/me/planning", agents(f.addProvider(bob), "test-chat", true), bob).Expect(http.StatusOK, "")
	f.Do("POST", f.path("/plans/%d/retry", pl.ID), nil, bob).Expect(http.StatusOK, "")
	f.work()
	if n := len(f.llm.Chats()); n != 0 {
		t.Fatalf("%d chat requests with the retrier's model", n)
	}
	f.Do("PUT", "/api/me/planning", agents(f.provider, "test-chat", true), f.ada).Expect(http.StatusOK, "")
	f.act(pl.ID, "retry", nil).Expect(http.StatusOK, "")
	f.llm.Reply(reply(ask("Who exports?")))
	f.work()
	if pl = f.plan(pl.ID); pl.Error != nil || len(pl.Messages) != 1 {
		t.Fatalf("plan after the creator picked a model %+v", pl)
	}
}
