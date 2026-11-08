package planning_test

import (
	"net/http"
	"testing"
)

func TestApproveProposal(t *testing.T) {
	f := setup(t)
	bob := f.member("bob", "push")
	pl := f.generate(f.approved(), task("Export endpoint", nil, script("go test ./...")), task("Export button", []int{1}, script("npm test")))

	// The second first, as bob: its issue names the first by title until that has its task.
	got := decode(t, f.review(bob, pl, 2, "approve", nil), http.StatusOK)
	if got.Status != "approved" || got.Version != pl.Version+1 || got.Proposals[0].Status != "needs_review" || got.Proposals[1].Status != "approved" {
		t.Fatalf("plan %+v", got)
	}
	f.expectTask(got.Proposals[1], 1, "bob", "Build Export button.\n\n## Acceptance criteria\n- [ ] Export button works\n\n"+
		"Depends on Export endpoint\n\n## Tests\n- [ ] `npm test`\n")
	f.review(f.ada, pl, 2, "approve", nil).Expect(http.StatusConflict, "wrong_status")

	if got = decode(t, f.review(f.ada, pl, 1, "approve", nil), http.StatusOK); got.Status != "done" || got.Pending != nil {
		t.Fatalf("plan %+v, want it done", got)
	}
	f.expectTask(got.Proposals[0], 2, "ada", "Build Export endpoint.\n\n## Acceptance criteria\n- [ ] Export endpoint works\n\n"+
		"## Tests\n- [ ] `go test ./...`\n")
	f.review(f.ada, pl, 1, "approve", nil).Expect(http.StatusConflict, "wrong_status")

	f.Do("POST", f.path("/plans/%d/proposals/999/approve", pl.ID), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("POST", f.path("/plans/%d/proposals/nope/approve", pl.ID), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	other := f.start("Imports", "Import a CSV.")
	f.Do("POST", f.path("/plans/%d/proposals/%d/approve", other.ID, pl.Proposals[0].ID), nil, f.ada).Expect(http.StatusNotFound, "not_found")
}

func TestApproveProposalWhenGitHubFails(t *testing.T) {
	f := setup(t)
	pl := f.generate(f.approved(), task("Export", nil, script("make")))
	f.GitHub.Fail("POST /repositories/{id}/issues", http.StatusInternalServerError)
	f.review(f.ada, pl, 1, "approve", nil).Expect(http.StatusBadGateway, "github_error")
	f.GitHub.Fail("POST /repositories/{id}/issues", http.StatusForbidden)
	f.review(f.ada, pl, 1, "approve", nil).Expect(http.StatusForbidden, "forbidden")
	if got := f.plan(pl.ID); got.Version != pl.Version || got.Proposals[0].Status != "needs_review" || got.Proposals[0].TaskID != nil {
		t.Fatalf("plan %+v, want it unchanged", got)
	}
	if n := f.tasks(); n != 0 {
		t.Fatalf("%d tasks", n)
	}
}
