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
}
