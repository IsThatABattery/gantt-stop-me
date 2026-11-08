package planning

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// settings are a user's planning settings: whether they review a plan's doc, and the model, nil for none, and own
// instructions of each agent of the plans they start.
type settings struct {
	Review      bool  `json:"review"`
	Interviewer agent `json:"interviewer"`
	TaskAgent   agent `json:"taskAgent"`
}

// agent is one planning agent's settings.
type agent struct {
	ProviderID   *int64  `json:"providerId"`
	Model        *string `json:"model"`
	Instructions string  `json:"instructions"`
}

// showSettings answers the caller's planning settings, the defaults when they saved none.
func (p *Planning) showSettings(w http.ResponseWriter, r *http.Request) {
	s := settings{Review: true}
	err := p.DB.QueryRow(r.Context(), `
		select review, provider_id, model, interviewer_instructions, task_provider_id, task_model, task_instructions
		from planning_settings where user_id = $1`, accounts.UserID(r.Context())).
		Scan(&s.Review, &s.Interviewer.ProviderID, &s.Interviewer.Model, &s.Interviewer.Instructions,
			&s.TaskAgent.ProviderID, &s.TaskAgent.Model, &s.TaskAgent.Instructions)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}
