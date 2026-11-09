package board_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	apiboard "gantt-stop-me/api/internal/board"
	"gantt-stop-me/api/internal/github/githubtest"
)

func TestBranchName(t *testing.T) {
	for title, want := range map[string]string{
		"Establish centralized Tailwind theme tokens": "feature/28-establish-centralized-tailwind-theme",
		"  Fix: the *board*!  ":                       "feature/28-fix-the-board",
		"日本語":                                         "feature/28",
		strings.Repeat("a", 50):                       "feature/28-" + strings.Repeat("a", 40),
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

	// A task without a branch whose name GitHub already has takes that branch.
	f.exec(`update tasks set branch = null where project_id = $1`, f.project)
	f.move(f.ada, 1, "backlog", 0).Expect(http.StatusOK, "")
	res = f.move(f.ada, 1, "in-progress", 0)
	res.Expect(http.StatusOK, "")
	got = task{}
	res.JSON(&got)
	if got.Branch == nil || *got.Branch != "feature/1-theme-tokens" || len(f.repo.Branches()) != 1 {
		t.Fatalf("branch = %v, GitHub has %v", got.Branch, f.repo.Branches())
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

func TestReorderInProgressMakesNoBranch(t *testing.T) {
	f := setup(t, "One", "Two")
	// Moved to In Progress on GitHub, so the sync puts them there without a branch.
	for _, n := range []int{1, 2} {
		f.repo.EditIssue(n, func(is *githubtest.Issue) { is.Labels = []string{"board:in-progress"} })
	}
	f.dueSync()
	if got := f.order("in-progress"); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("in-progress = %v", got)
	}
	n := len(f.GitHub.Requests())
	f.move(f.ada, 2, "in-progress", 1).Expect(http.StatusOK, "")
	if got := f.since(n); len(got) != 0 {
		t.Fatalf("a reorder within In Progress called GitHub: %v", got)
	}
}
