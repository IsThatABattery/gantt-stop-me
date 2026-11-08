package automation_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

type workflowsFile struct {
	Content   string
	SHA       *string
	Workflows []struct{ Name, Description string }
	Error     *string
}

const frontendFile = "workflows:\n  frontend:\n    description: React UI\n    context: src/front\n"

func TestWorkflowsFile(t *testing.T) {
	f := setup(t)
	read := func() workflowsFile {
		res := f.Do("GET", f.path("/workflows"), nil, f.ada)
		res.Expect(http.StatusOK, "")
		var got workflowsFile
		res.JSON(&got)
		return got
	}

	// No file yet: runs use the one default workflow.
	if got := read(); got.SHA != nil || got.Content != "" || len(got.Workflows) != 1 || got.Workflows[0].Name != "default" {
		t.Fatalf("without a file: %+v", got)
	}

	// A file that does not parse is refused before GitHub sees it.
	f.Do("PUT", f.path("/workflows"), map[string]any{"content": "workflows: {}\n"}, f.ada).Expect(http.StatusBadRequest, "invalid_workflows")

	// Saving commits to a new branch and opens a pull request; the default branch keeps its file until it merges.
	res := f.Do("PUT", f.path("/workflows"), map[string]any{"content": frontendFile, "sha": nil}, f.ada)
	res.Expect(http.StatusOK, "")
	var saved struct {
		Branch      string
		PullRequest struct {
			Number int
			URL    string
		}
	}
	res.JSON(&saved)
	if got, _ := f.repo.BranchFile(saved.Branch, ".gantt/workflows.yml"); got != frontendFile || saved.PullRequest.Number == 0 ||
		!strings.HasPrefix(saved.Branch, "chore/gantt-workflows-") {
		t.Fatalf("saved %+v, branch file %q", saved, got)
	}
	if pr := f.repo.Issue(saved.PullRequest.Number); pr.Title != "Update .gantt/workflows.yml" || pr.Pull == nil {
		t.Fatalf("pull request = %+v", pr)
	}
	if got := read(); got.SHA != nil {
		t.Fatalf("the default branch changed: %+v", got)
	}

	// Saving over a file changed since it was loaded is refused.
	f.repo.AddFile("", ".gantt/workflows.yml", frontendFile)
	f.Do("PUT", f.path("/workflows"), map[string]any{"content": frontendFile, "sha": nil}, f.ada).Expect(http.StatusConflict, "workflows_changed")
	sha := *read().SHA

	// Readers can see the file but not change it.
	bob, _, _ := f.member("bob", "pull")
	f.Do("GET", f.path("/workflows"), nil, bob).Expect(http.StatusOK, "")
	f.Do("PUT", f.path("/workflows"), map[string]any{"content": frontendFile, "sha": sha}, bob).Expect(http.StatusForbidden, "forbidden")
}

func TestSaveWorkflowsTwiceInOneSecond(t *testing.T) {
	f := setup(t)
	for time.Now().Nanosecond() > 200e6 { // early in a second, so both saves fall in it
		time.Sleep(10 * time.Millisecond)
	}
	body := map[string]any{"content": frontendFile, "sha": nil}
	f.Do("PUT", f.path("/workflows"), body, f.ada).Expect(http.StatusOK, "")
	// Each save gets a branch of its own, so the second is not told the file changed.
	f.Do("PUT", f.path("/workflows"), body, f.ada).Expect(http.StatusOK, "")
	if b := f.repo.Branches(); len(b) != 2 {
		t.Fatalf("branches = %v, want one per save", b)
	}
}

func TestSaveWorkflowsFailedLeavesNoBranch(t *testing.T) {
	f := setup(t)
	// Refused: the file appeared on the default branch since it was loaded.
	f.repo.AddFile("", ".gantt/workflows.yml", frontendFile)
	f.Do("PUT", f.path("/workflows"), map[string]any{"content": frontendFile, "sha": nil}, f.ada).Expect(http.StatusConflict, "workflows_changed")
	if b := f.repo.Branches(); len(b) != 0 {
		t.Fatalf("a refused save left branches %v", b)
	}

	var shown workflowsFile
	f.Do("GET", f.path("/workflows"), nil, f.ada).JSON(&shown)
	f.GitHub.Fail("POST /repositories/{id}/pulls", http.StatusUnprocessableEntity)
	f.Do("PUT", f.path("/workflows"), map[string]any{"content": frontendFile, "sha": shown.SHA}, f.ada).Expect(http.StatusBadGateway, "github_error")
	if b := f.repo.Branches(); len(b) != 0 {
		t.Fatalf("a save whose pull request failed left branches %v", b)
	}

	f.GitHub.Fail("PUT /repositories/{id}/contents/{path...}", http.StatusInternalServerError)
	f.Do("PUT", f.path("/workflows"), map[string]any{"content": frontendFile, "sha": shown.SHA}, f.ada).Expect(http.StatusBadGateway, "github_error")
	if b := f.repo.Branches(); len(b) != 0 {
		t.Fatalf("a save whose file write failed left branches %v", b)
	}
}
