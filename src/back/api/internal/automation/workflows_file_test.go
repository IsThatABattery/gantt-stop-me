package automation_test

import (
	"net/http"
	"testing"
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

	res := f.Do("PUT", f.path("/workflows"), map[string]any{"content": frontendFile, "sha": nil}, f.ada)
	res.Expect(http.StatusOK, "")
	var saved workflowsFile
	res.JSON(&saved)
	if saved.SHA == nil || len(saved.Workflows) != 1 || saved.Workflows[0].Name != "frontend" || saved.Workflows[0].Description != "React UI" {
		t.Fatalf("saved: %+v", saved)
	}
	if got := read(); got.Content != frontendFile || *got.SHA != *saved.SHA {
		t.Fatalf("read back: %+v", got)
	}

	// Saving over a file changed since it was loaded is refused.
	f.Do("PUT", f.path("/workflows"), map[string]any{"content": frontendFile, "sha": nil}, f.ada).Expect(http.StatusConflict, "workflows_changed")

	// Readers can see the file but not change it.
	bob, _, _ := f.member("bob", "pull")
	f.Do("GET", f.path("/workflows"), nil, bob).Expect(http.StatusOK, "")
	f.Do("PUT", f.path("/workflows"), map[string]any{"content": frontendFile, "sha": *saved.SHA}, bob).Expect(http.StatusForbidden, "forbidden")
}
