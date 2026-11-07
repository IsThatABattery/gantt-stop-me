// Package app wires every feature's routes into the API's one handler.
package app

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/board"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/secret"
)

// Config is everything the API reads from the environment, besides the database.
type Config struct {
	PublicURL          string         // base URL users open, without a trailing slash; the only Origin allowed to write
	Secrets            *secret.Box    // encrypts stored credentials
	GitHub             *github.Client // GitHub's API and OAuth endpoints
	GitHubClientID     string         // OAuth App used to link accounts; linking with OAuth is off while either is empty
	GitHubClientSecret string
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
	return httpx.CheckOrigin(cfg.PublicURL, mux)
}
