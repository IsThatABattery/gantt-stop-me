package settings

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// create adds a provider once its endpoint's model listing accepts the key and offers every chosen model.
func (s *Settings) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    string   `json:"name"`
		BaseURL string   `json:"baseUrl"`
		APIKey  string   `json:"apiKey"`
		Models  []string `json:"models"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		httpx.Error(w, http.StatusBadRequest, "invalid_name", "Name must be 1 to 100 characters.")
		return
	}
	base, ok := baseURL(in.BaseURL)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_base_url", "Base URL must be an absolute http or https URL.")
		return
	}
	key, ok := apiKey(in.APIKey)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_api_key", "API key must be non-empty and contain no control characters.")
		return
	}
	if in.Models == nil {
		in.Models = []string{}
	}
	available, ok := s.checkModels(w, r, base, key, in.Models)
	if !ok {
		return
	}

	p := provider{Name: name, BaseURL: base, KeyLast4: last4(key), Models: in.Models}
	err := s.db.QueryRow(r.Context(),
		`insert into providers (user_id, name, base_url, key_enc, models) values ($1, $2, $3, $4, $5) returning id`,
		accounts.UserID(r.Context()), name, base, s.secrets.Seal([]byte(key)), in.Models).Scan(&p.ID)
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23505" {
		httpx.Error(w, http.StatusConflict, "name_taken", "You already have a provider with this name.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, withListing{p, available})
}
