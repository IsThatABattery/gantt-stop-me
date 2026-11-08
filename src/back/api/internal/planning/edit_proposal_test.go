package planning_test

import (
	"net/http"
	"slices"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/tasktests"
)

// editProposal patches plan pl's proposal at position with body as ada.
func (f *fixture) editProposal(pl planJSON, position int, body map[string]any) *apptest.Response {
	f.t.Helper()
	return f.Do("PATCH", f.path("/plans/%d/proposals/%d", pl.ID, pl.Proposals[position-1].ID), body, f.ada)
}

func TestEditProposal(t *testing.T) {
	f := setup(t)
	pl := f.generate(f.approved(), task("Export endpoint", nil, script("go test ./...")), task("Export button", []int{1}, script("npm test")))
	tests := []tasktests.Test{script("npm test"), script("npm run lint")}
	got := decode(t, f.editProposal(pl, 2, map[string]any{"title": " CSV button ", "tests": tests, "dependsOn": []int{}, "version": pl.Version}),
		http.StatusOK)
	if p := got.Proposals[1]; p.Title != "CSV button" || p.Description != "Build Export button." || !slices.Equal(p.Tests, tests) ||
		len(p.DependsOn) != 0 || p.Status != "needs_review" || got.Version != pl.Version+1 {
		t.Fatalf("plan after the edit %+v", got)
	}

	res := f.editProposal(pl, 2, map[string]any{"title": "Lost", "version": pl.Version})
	res.Expect(http.StatusConflict, "conflict")
	var conflict struct{ Plan planJSON }
	if res.JSON(&conflict); conflict.Plan.Proposals[1].Title != "CSV button" {
		t.Fatalf("conflict %s, want the plan as it is", res.Body)
	}
	for _, edit := range []map[string]any{
		{"title": " "},
		{"title": "Two\nlines"},
		{"description": "```\nunclosed"},
		{"acceptanceCriteria": []string{"Works", ""}},
		{"acceptanceCriteria": []string{}},
		{"tests": []tasktests.Test{}},
		{"tests": []tasktests.Test{{Name: "Builds", Kind: "script"}}},
		{"dependsOn": []int{2}},
	} {
		edit["version"] = got.Version
		f.editProposal(got, 2, edit).Expect(http.StatusBadRequest, "invalid_proposal")
	}
	f.editProposal(got, 2, map[string]any{"title": "Lost", "version": "1"}).Expect(http.StatusBadRequest, "bad_request")

	// Approving it records the edit.
	got = decode(t, f.review(f.ada, got, 2, "approve", nil), http.StatusOK)
	if got.Proposals[1].Status != "approved_edited" {
		t.Fatalf("proposal %+v, want it approved_edited", got.Proposals[1])
	}
	f.expectTask(got.Proposals[1], 1, "ada", "Build Export button.\n\n## Acceptance criteria\n- [ ] Export button works\n\n"+
		"## Tests\n- [ ] `npm test`\n- [ ] `npm run lint`\n")
	f.editProposal(got, 2, map[string]any{"title": "Late", "version": got.Version}).Expect(http.StatusConflict, "wrong_status")
}
