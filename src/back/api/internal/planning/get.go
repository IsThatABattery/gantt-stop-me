package planning

import (
	"net/http"

	"gantt-stop-me/api/internal/projects"
)

// get answers one of the project's plans with its messages.
func (p *Planning) get(w http.ResponseWriter, r *http.Request) {
	pr, _, ok := p.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
