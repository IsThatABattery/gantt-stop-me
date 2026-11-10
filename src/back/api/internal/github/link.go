package github

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/oauth"
)

const (
	callbackPath = "/api/github/callback"
	// scope lets the token read and write the repositories, and merge pull requests that change .github/workflows,
	// which GitHub refuses an OAuth token without workflow.
	scope = "repo workflow"
)

// link starts linking with OAuth: it sends the browser to GitHub with a state tied to the session, which the callback checks.
func (l *Links) link(w http.ResponseWriter, r *http.Request) {
	if !l.oauthConfigured() {
		l.settings(w, r, "unavailable")
		return
	}
	session, _ := r.Cookie(accounts.SessionCookie) // RequireOr guarantees it
	l.authorize(w, r, oauth.Link, session.Value, scope)
}

// authorize sends the browser to GitHub to grant scope, with a new state for a flow of kind, bound to session or,
// when "", to the browser alone. GitHub sends the browser back to the callback.
func (l *Links) authorize(w http.ResponseWriter, r *http.Request, kind oauth.Kind, session, scope string) {
	q := url.Values{
		"client_id":    {l.ClientID},
		"redirect_uri": {l.PublicURL + callbackPath},
		"scope":        {scope},
		"state":        {l.stateCookie().Issue(w, kind, session)},
	}
	http.Redirect(w, r, l.Client.Web+"/login/oauth/authorize?"+q.Encode(), http.StatusFound)
}

// signedOut sends a browser that link or the callback cannot tie to a session back to Settings with the error
// outcome, dropping any OAuth state.
func (l *Links) signedOut(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, accounts.ErrSignedOut) {
		log.Printf("%s %s: no valid session; the browser must be signed in at %s", r.Method, r.URL.Path, l.PublicURL)
	} else {
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	}
	l.stateCookie().Clear(w)
	l.settings(w, r, "error")
}

// settings sends the browser back to Settings with the outcome of linking with OAuth, at its GitHub section, which
// shows the outcome.
func (l *Links) settings(w http.ResponseWriter, r *http.Request, outcome string) {
	http.Redirect(w, r, l.PublicURL+"/settings?github="+outcome+"#github", http.StatusFound)
}

// stateCookie is the github_state cookie, which carries the OAuth state to the callback alone.
func (l *Links) stateCookie() oauth.StateCookie {
	return oauth.StateCookie{Name: "github_state", Path: callbackPath, Secure: strings.HasPrefix(l.PublicURL, "https://")}
}
