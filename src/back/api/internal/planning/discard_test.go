package planning_test

import (
	"context"
	"net/http"
	"testing"
)

func TestDiscard(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.Do("DELETE", f.path("/plans/%d", pl.ID), nil, f.ada).Expect(http.StatusNoContent, "")
	if pl = f.plan(pl.ID); pl.Status != "discarded" || pl.Pending != nil {
		t.Fatalf("discarded plan %+v", pl)
	}
	f.work()
	if n := len(f.llm.Chats()); n != 0 {
		t.Fatalf("%d chat requests for a discarded plan", n)
	}
	f.act(pl.ID, "draft", nil).Expect(http.StatusConflict, "wrong_status")
	f.Do("DELETE", f.path("/plans/%d", pl.ID), nil, f.ada).Expect(http.StatusNoContent, "")
	f.Do("DELETE", f.path("/plans/999"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
}

// While the agent's turn holds a plan, changing it answers at once instead of waiting for the turn.
func TestChangesDuringATurnAnswerBusy(t *testing.T) {
	f := setup(t)
	pl := f.toReview()
	ctx := context.Background()
	tx, err := f.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select from feature_plans where id = $1 for update`, pl.ID); err != nil {
		t.Fatal(err)
	}
	f.Do("DELETE", f.path("/plans/%d", pl.ID), nil, f.ada).Expect(http.StatusConflict, "agent_busy")
	f.act(pl.ID, "approve", nil).Expect(http.StatusConflict, "agent_busy")
	f.Do("GET", f.path("/plans/%d", pl.ID), nil, f.ada).Expect(http.StatusOK, "")
	tx.Rollback(ctx)
	f.act(pl.ID, "approve", nil).Expect(http.StatusOK, "")
}
