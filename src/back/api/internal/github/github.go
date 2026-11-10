// Package github links users' GitHub accounts, signs users in with them, and calls GitHub with their credentials.
package github

import (
	"net/http"

	"gantt-stop-me/api/internal/accounts"
)

// Links serves the endpoints that link and unlink a user's GitHub account, and sign in with it.
type Links struct {
	*Credentials                    // stores the links
	Accounts     *accounts.Accounts // guards every route but sign-in's, and starts its sessions
	PublicURL    string             // base URL users open, without a trailing slash
}

// Register adds the GitHub link and sign-in routes to mux.
func (l *Links) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/github/oauth", l.oauth) // public: sign-in asks it too
	// The browser navigates to these, so they answer with redirects only: to Settings for a link, to sign-in for a
	// sign-in. The callback finishes both.
	mux.HandleFunc("GET /api/github/link", l.Accounts.RequireOr(l.signedOut, l.link))
	mux.HandleFunc("GET /api/auth/github", l.signIn)
	mux.HandleFunc("GET "+callbackPath, l.callback)
	mux.HandleFunc("POST /api/github/token", l.Accounts.Require(l.token))
	mux.HandleFunc("DELETE /api/github", l.Accounts.Require(l.unlink))
}
