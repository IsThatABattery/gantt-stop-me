package board

import (
	"net/http"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// assignable lists the people the project's tasks can be assigned to, as GitHub has them for the repository.
func (b *Board) assignable(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	users, err := b.GitHub.Assignable(r.Context(), token, p.RepoID)
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	out := make([]projects.Assignee, len(users))
	for i, u := range users {
		out[i] = projects.Assignee{Login: u.Login, AvatarURL: u.AvatarURL}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"assignees": out})
}
