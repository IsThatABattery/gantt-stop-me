package planning

import (
	"net/http"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// create starts a plan in briefing with the agent's first question as its job. Its agent uses the caller's
// planning settings for good, so the caller needs a planning model.
func (p *Planning) create(w http.ResponseWriter, r *http.Request) {
	pr, _, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	title, ok := text(in.Title, 256)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_title", "A title needs 1 to 256 characters.")
		return
	}
	description, ok := text(in.Description, maxText)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_description", "A description needs 1 to 10000 characters.")
		return
	}
	ctx, userID := r.Context(), accounts.UserID(r.Context())
	var set bool
	err := p.DB.QueryRow(ctx, `select exists (select from planning_settings where user_id = $1 and model is not null)`, userID).Scan(&set)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if !set {
		httpx.Error(w, http.StatusConflict, "planning_not_set_up", "Pick a planning model in Settings first.")
		return
	}
	var id int64
	err = p.DB.QueryRow(ctx, `
		insert into feature_plans (project_id, created_by, title, description, status, doc, pending)
		values ($1, $2, $3, $4, 'briefing', $5, 'ask') returning id`, pr.ID, userID, title, description, topics).Scan(&id)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	p.show(w, r, http.StatusCreated, pr.ID, id)
}
