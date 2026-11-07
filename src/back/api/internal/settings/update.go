package settings

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// update changes a provider's base URL, key or models after re-fetching its model listing.
// An empty body changes nothing and returns the current listing: that is how the UI refreshes model choices.
func (s *Settings) update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BaseURL *string  `json:"baseUrl"`
		APIKey  *string  `json:"apiKey"`
		Models  []string `json:"models"` // nil keeps the stored models
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	ctx, userID := r.Context(), accounts.UserID(r.Context())
	p := provider{ID: providerID(r)}
	var keyEnc []byte
	err := s.db.QueryRow(ctx, `select name, base_url, key_enc, models from providers where id = $1 and user_id = $2`,
		p.ID, userID).Scan(&p.Name, &p.BaseURL, &keyEnc, &p.Models)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "Provider not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}

	if in.BaseURL != nil {
		base, ok := baseURL(*in.BaseURL)
		if !ok {
			httpx.Error(w, http.StatusBadRequest, "invalid_base_url", "Base URL must be an absolute http or https URL.")
			return
		}
		p.BaseURL = base
	}
	var key string
	switch {
	case in.APIKey == nil:
		plain, err := s.secrets.Open(keyEnc)
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		key = string(plain)
	case *in.APIKey == "":
		httpx.Error(w, http.StatusBadRequest, "invalid_api_key", "Enter an API key.")
		return
	default:
		key, keyEnc = *in.APIKey, s.secrets.Seal([]byte(*in.APIKey))
	}
	// Only models sent in this request are checked, so a refresh still works after the endpoint drops a stored one.
	available, ok := checkModels(w, r, p.BaseURL, key, in.Models)
	if !ok {
		return
	}
	if in.Models != nil {
		p.Models = in.Models
	}

	tag, err := s.db.Exec(ctx, `update providers set base_url = $3, key_enc = $4, models = $5 where id = $1 and user_id = $2`,
		p.ID, userID, p.BaseURL, keyEnc, p.Models)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 { // deleted while the listing was fetched
		httpx.Error(w, http.StatusNotFound, "not_found", "Provider not found.")
		return
	}
	p.KeyLast4 = last4(key)
	httpx.JSON(w, http.StatusOK, withListing{p, available})
}
