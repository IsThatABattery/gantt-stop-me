package automation

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

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

// saveWorkflows answers PUT /api/projects/{id}/workflows: it commits a workflows file that parses to a new branch off
// the default branch, and opens a pull request of it, with the caller's GitHub credential.
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
	// On its own branch, with a pull request, so the change is reviewed and tested like any other. The random part keeps
	// another save, or a branch someone made ahead, from sharing it.
	var nonce [4]byte
	rand.Read(nonce[:])
	branch := fmt.Sprintf("chore/gantt-workflows-%d-%x", time.Now().Unix(), nonce)
	base, err := a.GitHub.CreateBranch(r.Context(), token, p.RepoID, branch)
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	// A save that fails takes its branch with it, so nothing is left saved.
	drop := func() {
		if err := a.GitHub.DeleteBranch(context.WithoutCancel(r.Context()), token, p.RepoID, branch); err != nil {
			log.Printf("delete branch %s: %v", branch, err)
		}
	}
	_, err = a.GitHub.PutWorkflows(r.Context(), token, p.RepoID, branch, in.Content, sha, "Update "+github.WorkflowsPath)
	var ghErr *github.Error
	// 409: sha is no longer the file's. 422 without a sha: the file appeared since it was loaded.
	if errors.As(err, &ghErr) && (ghErr.Status == http.StatusConflict || ghErr.Status == http.StatusUnprocessableEntity && in.SHA == nil) {
		drop()
		httpx.Error(w, http.StatusConflict, "workflows_changed", "The workflows file changed on GitHub since it was loaded. Reload it and save again.")
		return
	}
	if err != nil {
		drop()
		github.Fail(w, r, err)
		return
	}
	number, url, err := a.GitHub.CreatePullRequest(r.Context(), token, p.RepoID, "Update "+github.WorkflowsPath,
		"Changes the workflows that test runs build, from the board's Automation page.", branch, base)
	if err != nil {
		drop()
		github.Fail(w, r, err)
		return
	}
	// The default branch keeps its file until the pull request merges.
	httpx.JSON(w, http.StatusOK, map[string]any{"branch": branch, "pullRequest": map[string]any{"number": number, "url": url}})
}
