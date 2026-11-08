package automation

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/projects"
)

// enable turns automation on with the caller as its owner, or takes it over: the caller replaces the owner and
// models and unpauses it, and the webhook secret and the poller's ETag stay.
func (a *Automation) enable(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		DecisionProviderID   int64  `json:"decisionProviderId"`
		DecisionModel        string `json:"decisionModel"`
		ChatProviderID       int64  `json:"chatProviderId"`
		ChatModel            string `json:"chatModel"`
		DecisionInstructions string `json:"decisionInstructions"`
		ChatInstructions     string `json:"chatInstructions"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	ctx := r.Context()
	userID := accounts.UserID(ctx)
	if !a.chosenModel(w, r, userID, in.DecisionProviderID, in.DecisionModel) || !a.chosenModel(w, r, userID, in.ChatProviderID, in.ChatModel) {
		return
	}
	var whyDecision, whyChat string
	in.DecisionInstructions, whyDecision = llm.Instructions(in.DecisionInstructions)
	in.ChatInstructions, whyChat = llm.Instructions(in.ChatInstructions)
	if why := cmp.Or(whyDecision, whyChat); why != "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_instructions", why)
		return
	}
	var s settings
	// One transaction, so the answer is this write's row: a concurrent PUT or DELETE waits on its row lock.
	err := pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			insert into project_automation (project_id, owner_id, decision_provider, decision_model, chat_provider, chat_model,
				decision_instructions, chat_instructions)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
			on conflict (project_id) do update set owner_id = excluded.owner_id,
				decision_provider = excluded.decision_provider, decision_model = excluded.decision_model,
				chat_provider = excluded.chat_provider, chat_model = excluded.chat_model,
				decision_instructions = excluded.decision_instructions, chat_instructions = excluded.chat_instructions,
				paused_reason = null`,
			p.ID, userID, in.DecisionProviderID, in.DecisionModel, in.ChatProviderID, in.ChatModel,
			in.DecisionInstructions, in.ChatInstructions); err != nil {
			return err
		}
		var err error
		s, err = a.settings(ctx, tx, p.ID)
		return err
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// chosenModel checks the provider is the user's and model is one of the models they chose on it, answering 400 when not.
func (a *Automation) chosenModel(w http.ResponseWriter, r *http.Request, userID, providerID int64, model string) bool {
	var models []string
	err := a.DB.QueryRow(r.Context(), `select models from providers where id = $1 and user_id = $2`, providerID, userID).Scan(&models)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusBadRequest, "invalid_provider", "Pick one of your own providers from Settings.")
		return false
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return false
	}
	if !slices.Contains(models, model) {
		httpx.Error(w, http.StatusBadRequest, "invalid_model", fmt.Sprintf("Model %q is not one of this provider's chosen models.", model))
		return false
	}
	return true
}
