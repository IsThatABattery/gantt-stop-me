package github

import (
	"crypto/rand"
	"net/http"
	"net/url"
	"strings"

	"gantt-stop-me/api/internal/httpx"
)

const (
	callbackPath = "/api/github/callback"
	stateCookie  = "github_state"
)

// link starts linking with OAuth: it sends the browser to GitHub with a state the callback checks.
func (l *Links) link(w http.ResponseWriter, r *http.Request) {
	if l.ClientID == "" || l.ClientSecret == "" {
		httpx.Error(w, http.StatusServiceUnavailable, "oauth_not_configured", "Linking with OAuth is not set up on this server. Add a personal access token instead.")
		return
	}
	state := rand.Text()
	http.SetCookie(w, l.cookie(state, 600))
	q := url.Values{
		"client_id":    {l.ClientID},
		"redirect_uri": {l.PublicURL + callbackPath},
		"scope":        {"repo"},
		"state":        {state},
	}
	http.Redirect(w, r, l.Client.Web+"/login/oauth/authorize?"+q.Encode(), http.StatusFound)
}

// cookie builds the OAuth state cookie, sent only to the callback; maxAge < 0 deletes it.
func (l *Links) cookie(state string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     callbackPath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   strings.HasPrefix(l.PublicURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	}
}
