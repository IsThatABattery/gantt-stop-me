package planning

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/llm"
)

// saveSettings replaces the caller's planning settings. An agent left out, or with a null provider and model, has
// none, so the caller can delete the provider; review left out is on.
func (p *Planning) saveSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Review      *bool `json:"review"`
		Interviewer agent `json:"interviewer"`
		TaskAgent   agent `json:"taskAgent"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	ctx, userID := r.Context(), accounts.UserID(r.Context())
	if !p.validAgent(w, r, userID, &in.Interviewer) || !p.validAgent(w, r, userID, &in.TaskAgent) {
		return
	}
	s := settings{Review: in.Review == nil || *in.Review, Interviewer: in.Interviewer, TaskAgent: in.TaskAgent}
	i, t := s.Interviewer, s.TaskAgent
	if _, err := p.DB.Exec(ctx, `
		insert into planning_settings (user_id, review, provider_id, model, interviewer_instructions,
			task_provider_id, task_model, task_instructions) values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (user_id) do update set review = excluded.review, provider_id = excluded.provider_id,
			model = excluded.model, interviewer_instructions = excluded.interviewer_instructions,
			task_provider_id = excluded.task_provider_id, task_model = excluded.task_model,
			task_instructions = excluded.task_instructions`,
		userID, s.Review, i.ProviderID, i.Model, i.Instructions, t.ProviderID, t.Model, t.Instructions); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// validAgent checks a's provider is the user's and its model one of the provider's chosen models, or both are nil,
// and trims its instructions, answering 400 when any is wrong.
func (p *Planning) validAgent(w http.ResponseWriter, r *http.Request, userID int64, a *agent) bool {
	if a.ProviderID != nil || a.Model != nil {
		var models []string
		err := pgx.ErrNoRows
		if a.ProviderID != nil {
			err = p.DB.QueryRow(r.Context(), `select models from providers where id = $1 and user_id = $2`, *a.ProviderID, userID).Scan(&models)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusBadRequest, "invalid_provider", "Pick one of your own providers from Settings.")
			return false
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return false
		}
		if a.Model == nil || !slices.Contains(models, *a.Model) {
			httpx.Error(w, http.StatusBadRequest, "invalid_model", "Pick one of this provider's chosen models.")
			return false
		}
	}
	var ok bool
	if a.Instructions, ok = llm.Instructions(a.Instructions); !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_instructions", fmt.Sprintf("Instructions can have at most %d characters.", llm.MaxInstructions))
		return false
	}
	return true
}
