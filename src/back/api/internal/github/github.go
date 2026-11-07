// Package github links users' GitHub accounts and calls GitHub with their credentials.
package github

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/secret"
)

// Links serves the endpoints that link and unlink a user's GitHub account.
type Links struct {
	DB           *pgxpool.Pool
	Accounts     *accounts.Accounts // guards every route
	Secrets      *secret.Box        // encrypts stored credentials
	Client       *Client
	ClientID     string // OAuth App; linking with OAuth answers 503 while either is empty
	ClientSecret string
	PublicURL    string // base URL users open, without a trailing slash
}

// Register adds the GitHub link routes to mux.
func (l *Links) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/github/link", l.Accounts.Require(l.link))
	mux.HandleFunc("GET "+callbackPath, l.Accounts.Require(l.callback))
	mux.HandleFunc("POST /api/github/token", l.Accounts.Require(l.token))
	mux.HandleFunc("DELETE /api/github", l.Accounts.Require(l.unlink))
}
