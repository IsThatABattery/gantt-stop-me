// Package app wires every feature's routes into the API's one handler.
package app

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/secret"
	"gantt-stop-me/api/internal/settings"
)

// Config is everything the API reads from the environment, besides the database.
type Config struct {
	PublicURL string      // base URL users open, without a trailing slash; the only Origin allowed to write
	Secrets   *secret.Box // encrypts stored credentials
}

// New returns the API handler.
func New(db *pgxpool.Pool, cfg Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health)
	auth := accounts.New(db, cfg.PublicURL)
	auth.Register(mux)
	settings.New(db, cfg.Secrets, auth).Register(mux)
	return httpx.CheckOrigin(cfg.PublicURL, mux)
}
