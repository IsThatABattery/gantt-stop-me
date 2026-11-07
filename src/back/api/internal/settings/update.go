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

	// Only what this request changes is written (nil keeps the column), so an overlapping PATCH's changes survive.
	var newBase *string
	var newKeyEnc []byte
	if in.BaseURL != nil {
		base, ok := baseURL(*in.BaseURL)
		if !ok {
			httpx.Error(w, http.StatusBadRequest, "invalid_base_url", "Base URL must be an absolute http or https URL.")
			return
		}
		if base != p.BaseURL {
			// The stored key goes only to the URL it was entered for, or a session holder could send it to any host.
			if in.APIKey == nil {
				httpx.Error(w, http.StatusBadRequest, "invalid_api_key", "Enter the API key again to change the base URL.")
				return
			}
			p.BaseURL, newBase = base, &base
		}
	}
	var key string
	if in.APIKey == nil {
		plain, err := s.secrets.Open(keyEnc)
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		key = string(plain)
	} else {
		k, ok := apiKey(*in.APIKey)
		if !ok {
			httpx.Error(w, http.StatusBadRequest, "invalid_api_key", "API key must be non-empty and contain no control characters.")
			return
		}
		key, newKeyEnc = k, s.secrets.Seal([]byte(k))
	}
	// Only models sent in this request are checked, so a refresh still works after the endpoint drops a stored one.
	available, ok := s.checkModels(w, r, p.BaseURL, key, in.Models)
	if !ok {
		return
	}

	if newBase != nil || newKeyEnc != nil || in.Models != nil {
		err = s.db.QueryRow(ctx, `update providers set base_url = coalesce($3, base_url), key_enc = coalesce($4, key_enc),
			models = coalesce($5, models) where id = $1 and user_id = $2 returning base_url, key_enc, models`,
			p.ID, userID, newBase, newKeyEnc, in.Models).Scan(&p.BaseURL, &keyEnc, &p.Models)
		if errors.Is(err, pgx.ErrNoRows) { // deleted while the listing was fetched
			httpx.Error(w, http.StatusNotFound, "not_found", "Provider not found.")
			return
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
	}
	// From the row as written, which may hold a key another PATCH rotated meanwhile.
	plain, err := s.secrets.Open(keyEnc)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	p.KeyLast4 = last4(string(plain))
	httpx.JSON(w, http.StatusOK, withListing{p, available})
}
