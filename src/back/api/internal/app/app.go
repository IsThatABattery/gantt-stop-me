// Package app wires every feature's routes into the API's one handler.
package app

import (
	"net/http"
	"net/http/httptest"

	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/automation"
	"gantt-stop-me/api/internal/board"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/secret"
	"gantt-stop-me/api/internal/settings"
)

// Config is everything the API reads from the environment, besides the database.
type Config struct {
	PublicURL          string         // base URL users open, without a trailing slash; the only Origin allowed to write
	Secrets            *secret.Box    // encrypts stored credentials
	GitHub             *github.Client // GitHub's API and OAuth endpoints
	GitHubClientID     string         // OAuth App used to link accounts; linking with OAuth is off while either is empty
	GitHubClientSecret string
	// AllowPrivateProviderURLs lets provider base URLs reach loopback and private networks, such as a local Ollama.
	AllowPrivateProviderURLs bool
	RunnerURL                string // the runner's base URL; automation is off, its routes 404, while it is empty
}

// New returns the API handler.
func New(db *pgxpool.Pool, cfg Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health)
	acc := accounts.New(db, cfg.PublicURL)
	acc.Register(mux)
	links := &github.Links{
		DB:           db,
		Accounts:     acc,
		Secrets:      cfg.Secrets,
		Client:       cfg.GitHub,
		ClientID:     cfg.GitHubClientID,
		ClientSecret: cfg.GitHubClientSecret,
		PublicURL:    cfg.PublicURL,
	}
	links.Register(mux)
	prj := &projects.Projects{DB: db, Accounts: acc, Secrets: cfg.Secrets, GitHub: cfg.GitHub}
	prj.Register(mux)
	(&board.Board{DB: db, Accounts: acc, Projects: prj, GitHub: cfg.GitHub}).Register(mux)
	settings.New(db, cfg.Secrets, acc, cfg.AllowPrivateProviderURLs).Register(mux)
	(&automation.Automation{
		DB:        db,
		Accounts:  acc,
		Projects:  prj,
		Secrets:   cfg.Secrets,
		GitHub:    cfg.GitHub,
		PublicURL: cfg.PublicURL,
		RunnerURL: cfg.RunnerURL,
	}).Register(mux)
	return httpx.CheckOrigin(cfg.PublicURL, jsonErrors(mux))
}

// jsonErrors answers the mux's own 404 and 405 in the API's one error shape.
func jsonErrors(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		// No route: run the mux's fallback only to learn what it would answer and which methods it allows.
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		switch rec.Code {
		case http.StatusMethodNotAllowed:
			w.Header().Set("Allow", rec.Header().Get("Allow"))
			httpx.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		case http.StatusNotFound:
			httpx.Error(w, http.StatusNotFound, "not_found", "No such endpoint.")
		default: // a redirect to the cleaned path
			mux.ServeHTTP(w, r)
		}
	})
}
