package github

import (
	"net/http"

	"gantt-stop-me/api/internal/httpx"
)

// oauth reports whether this server can link and sign in with OAuth, so Settings and sign-in offer it only then.
func (l *Links) oauth(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]bool{"enabled": l.oauthConfigured()})
}
