// Package settings owns a user's LLM providers: OpenAI-compatible endpoints with an encrypted key.
package settings

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/secret"
)

// Settings serves the /api/providers endpoints.
type Settings struct {
	db       *pgxpool.Pool
	secrets  *secret.Box
	accounts *accounts.Accounts
}

// New returns Settings that seal keys with secrets and guard every route with accounts.Require.
func New(db *pgxpool.Pool, secrets *secret.Box, accounts *accounts.Accounts) *Settings {
	return &Settings{db: db, secrets: secrets, accounts: accounts}
}

// Register adds the provider routes to mux.
func (s *Settings) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/providers", s.accounts.Require(s.list))
	mux.HandleFunc("POST /api/providers", s.accounts.Require(s.create))
	mux.HandleFunc("PATCH /api/providers/{providerID}", s.accounts.Require(s.update))
	mux.HandleFunc("DELETE /api/providers/{providerID}", s.accounts.Require(s.delete))
}

// provider is how the API shows a provider: never the key, only its last four characters.
type provider struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	BaseURL  string   `json:"baseUrl"`
	KeyLast4 string   `json:"keyLast4"`
	Models   []string `json:"models"`
}

// withListing is a provider and the models its endpoint offers right now.
type withListing struct {
	provider
	AvailableModels []string `json:"availableModels"`
}

func last4(key string) string {
	r := []rune(key)
	if len(r) <= 4 {
		return "" // the last four of a short key would be the whole key
	}
	return string(r[len(r)-4:])
}

// baseURL checks raw is an absolute http or https URL and returns it without a trailing slash.
func baseURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	// A query or fragment would swallow the /models path appended to it.
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.ContainsAny(raw, "?#") {
		return "", false
	}
	return strings.TrimRight(raw, "/"), true
}

// providerID is the {providerID} path value, or 0, which matches no provider, if it is not a number.
func providerID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("providerID"), 10, 64)
	return id
}
