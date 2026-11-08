package planning

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// settings are a user's planning settings: the model the agents of the plans they start use, nil for none, and
// whether they review a plan's doc.
type settings struct {
	ProviderID *int64  `json:"providerId"`
	Model      *string `json:"model"`
	Review     bool    `json:"review"`
}

// showSettings answers the caller's planning settings, the defaults when they saved none.
func (p *Planning) showSettings(w http.ResponseWriter, r *http.Request) {
	s := settings{Review: true}
	err := p.DB.QueryRow(r.Context(), `select provider_id, model, review from planning_settings where user_id = $1`,
		accounts.UserID(r.Context())).Scan(&s.ProviderID, &s.Model, &s.Review)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}
