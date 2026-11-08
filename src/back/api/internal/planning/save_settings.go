package planning

import (
	"errors"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// saveSettings replaces the caller's planning settings. A null provider and model pick none, so the caller can
// delete the provider; review left out is on.
func (p *Planning) saveSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProviderID *int64  `json:"providerId"`
		Model      *string `json:"model"`
		Review     *bool   `json:"review"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	ctx, userID := r.Context(), accounts.UserID(r.Context())
	if in.ProviderID != nil || in.Model != nil {
		var models []string
		err := pgx.ErrNoRows
		if in.ProviderID != nil {
			err = p.DB.QueryRow(ctx, `select models from providers where id = $1 and user_id = $2`, *in.ProviderID, userID).Scan(&models)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusBadRequest, "invalid_provider", "Pick one of your own providers from Settings.")
			return
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		if in.Model == nil || !slices.Contains(models, *in.Model) {
			httpx.Error(w, http.StatusBadRequest, "invalid_model", "Pick one of this provider's chosen models.")
			return
		}
	}
	s := settings{ProviderID: in.ProviderID, Model: in.Model, Review: in.Review == nil || *in.Review}
	if _, err := p.DB.Exec(ctx, `
		insert into planning_settings (user_id, provider_id, model, review) values ($1, $2, $3, $4)
		on conflict (user_id) do update set provider_id = excluded.provider_id, model = excluded.model, review = excluded.review`,
		userID, s.ProviderID, s.Model, s.Review); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}
