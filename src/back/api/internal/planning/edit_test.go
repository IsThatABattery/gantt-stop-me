package planning_test

import (
	"net/http"
	"reflect"
	"testing"
)

func TestEditDoc(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.Do("PATCH", f.path("/plans/%d", pl.ID), map[string]any{"doc": []any{}, "version": pl.Version}, f.ada).
		Expect(http.StatusConflict, "agent_busy")
	f.llm.Reply(reply(ask("Who exports?")))
	f.work()
	pl = f.plan(pl.ID)

	// The doc as GET answers it, with one body changed, saves that body; titles stay.
	doc := pl.Doc
	doc[0].Body, doc[0].Title = "Export the board.", "Renamed"
	doc[1].Body = []any{"Admins", "Leads"}
	got := decode(t, f.Do("PATCH", f.path("/plans/%d", pl.ID), map[string]any{"doc": doc, "version": pl.Version}, f.ada), http.StatusOK)
	if g, u := got.section("goal"), got.section("users"); got.Version != pl.Version+1 || g.Title != "Goal" ||
		g.Body != "Export the board." || g.State != "not_yet" || !reflect.DeepEqual(u.Body, []any{"Admins", "Leads"}) {
		t.Fatalf("edited plan %+v", got)
	}
	// A section alone, and a state alone.
	got = decode(t, f.Do("PATCH", f.path("/plans/%d", pl.ID), map[string]any{
		"doc": []map[string]any{{"key": "goal", "state": "covered"}}, "version": got.Version,
	}, f.ada), http.StatusOK)
	if g := got.section("goal"); g.Body != "Export the board." || g.State != "covered" || got.section("users").Body == "" {
		t.Fatalf("goal %+v after a state-only edit", g)
	}

	// A stale version answers the plan as it is now.
	res := f.Do("PATCH", f.path("/plans/%d", pl.ID), map[string]any{"doc": doc, "version": pl.Version}, f.ada)
	res.Expect(http.StatusConflict, "conflict")
	var conflict struct{ Plan planJSON }
	res.JSON(&conflict)
	if conflict.Plan.Version != got.Version || conflict.Plan.section("goal").State != "covered" || len(conflict.Plan.Messages) != 1 {
		t.Fatalf("conflict answered %+v", conflict.Plan)
	}

	for _, bad := range []map[string]any{
		{"key": "nope", "body": "x"},
		{"key": "goal", "body": 3},
		{"key": "goal", "body": []any{"a", nil}},
		{"key": "goal", "state": "done"},
	} {
		f.Do("PATCH", f.path("/plans/%d", pl.ID), map[string]any{"doc": []any{bad}, "version": got.Version}, f.ada).
			Expect(http.StatusBadRequest, "invalid_doc")
	}
	if now := f.plan(pl.ID); now.Version != got.Version {
		t.Fatalf("version %d after refused edits, want %d", now.Version, got.Version)
	}

	pl = f.toReview()
	f.Do("PATCH", f.path("/plans/%d", pl.ID), map[string]any{"doc": []any{}, "version": pl.Version}, f.ada).Expect(http.StatusOK, "")
	f.act(pl.ID, "approve", nil).Expect(http.StatusOK, "")
	f.Do("PATCH", f.path("/plans/%d", pl.ID), map[string]any{"doc": []any{}, "version": pl.Version + 2}, f.ada).
		Expect(http.StatusConflict, "wrong_status")
}
