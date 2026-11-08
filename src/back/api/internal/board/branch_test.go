package board_test

import (
	"net/http"
	"slices"
	"testing"

	apiboard "gantt-stop-me/api/internal/board"
)

func TestBranchName(t *testing.T) {
	for title, want := range map[string]string{
		"Establish centralized Tailwind theme tokens": "feature/28-establish-centralized-tailwind-theme",
		"  Fix: the *board*!  ":                       "feature/28-fix-the-board",
		"日本語":                                         "feature/28",
	} {
		if got := apiboard.BranchName(28, title); got != want {
			t.Errorf("BranchName(28, %q) = %q, want %q", title, got, want)
		}
	}
}

func TestMoveToInProgressMakesABranch(t *testing.T) {
	f := setup(t, "Theme tokens")
	res := f.move(f.ada, 1, "in-progress", 0)
	res.Expect(http.StatusOK, "")
	var got task
	res.JSON(&got)
	if got.Branch == nil || *got.Branch != "feature/1-theme-tokens" || !slices.Equal(f.repo.Branches(), []string{"feature/1-theme-tokens"}) {
		t.Fatalf("branch = %v, GitHub has %v", got.Branch, f.repo.Branches())
	}

	// Moving back in keeps the branch it has.
	f.move(f.ada, 1, "backlog", 0).Expect(http.StatusOK, "")
	f.move(f.ada, 1, "in-progress", 0).Expect(http.StatusOK, "")
	if b := f.repo.Branches(); len(b) != 1 {
		t.Fatalf("branches = %v", b)
	}
}

func TestMoveRecordsNoBranchGitHubRefused(t *testing.T) {
	f := setup(t, "Theme tokens")
	// GitHub answers 422 for any refused ref, such as one a ruleset blocks, not only for one that exists.
	f.GitHub.Fail("POST /repositories/{id}/git/refs", http.StatusUnprocessableEntity)
	res := f.move(f.ada, 1, "in-progress", 0)
	res.Expect(http.StatusOK, "")
	var got task
	res.JSON(&got)
	if got.Branch != nil {
		t.Fatalf("task records branch %q, GitHub has %v", *got.Branch, f.repo.Branches())
	}
}
