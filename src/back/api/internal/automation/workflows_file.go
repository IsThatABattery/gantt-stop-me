package automation

import (
	"errors"
	"net/http"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// workflowsFile is a repository's workflows file on its default branch, as the workflows routes answer it.
type workflowsFile struct {
	Content string  `json:"content"` // "" when the branch has no file
	SHA     *string `json:"sha"`     // the file's blob sha, to save over it; null when there is no file
	// Workflows is what runs use: the file's, or the one default workflow without a file. Empty when Error is set.
	Workflows []workflowSummary `json:"workflows"`
	Error     *string           `json:"error"` // why the file does not parse, else null
}

type workflowSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func newWorkflowsFile(content, sha string) workflowsFile {
	f := workflowsFile{Content: content, Workflows: []workflowSummary{}}
	ws := DefaultWorkflows()
	if sha != "" {
		f.SHA = &sha
		var err error
		if ws, err = ParseWorkflows([]byte(content)); err != nil {
			msg := err.Error()
			f.Error, ws = &msg, nil
		}
	}
	for _, w := range ws {
		f.Workflows = append(f.Workflows, workflowSummary{w.Name, w.Description})
	}
	return f
}

// showWorkflows answers GET /api/projects/{id}/workflows: the default branch's workflows file.
func (a *Automation) showWorkflows(w http.ResponseWriter, r *http.Request) {
	p, token, ok := a.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	content, sha, err := a.GitHub.DefaultWorkflows(r.Context(), token, p.RepoID)
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, newWorkflowsFile(content, sha))
}

// saveWorkflows answers PUT /api/projects/{id}/workflows: it commits a workflows file that parses to the default
// branch with the caller's GitHub credential.
func (a *Automation) saveWorkflows(w http.ResponseWriter, r *http.Request) {
	p, token, ok := a.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Content string  `json:"content"`
		SHA     *string `json:"sha"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if _, err := ParseWorkflows([]byte(in.Content)); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_workflows", err.Error())
		return
	}
	sha := ""
	if in.SHA != nil {
		sha = *in.SHA
	}
	sha, err := a.GitHub.PutDefaultWorkflows(r.Context(), token, p.RepoID, in.Content, sha, "Update "+github.WorkflowsPath)
	var ghErr *github.Error
	// 409: sha is no longer the file's. 422 without a sha: the file appeared since it was loaded.
	if errors.As(err, &ghErr) && (ghErr.Status == http.StatusConflict || ghErr.Status == http.StatusUnprocessableEntity && in.SHA == nil) {
		httpx.Error(w, http.StatusConflict, "workflows_changed", "The workflows file changed on GitHub since it was loaded. Reload it and save again.")
		return
	}
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, newWorkflowsFile(in.Content, sha))
}
