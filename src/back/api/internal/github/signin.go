package github

import (
	"net/http"
	"regexp"
	"strings"

	"gantt-stop-me/api/internal/oauth"
)

// signInScope is what linking asks for, and user:email, to read the verified email a new user is made with.
const signInScope = scope + " user:email"

// nextPath is a path on the app a sign-in may send the browser to: no host, query or fragment.
var nextPath = regexp.MustCompile(`^/[A-Za-z0-9/_-]*$`)

// signIn starts signing in with GitHub: it sends the browser to GitHub with a state that the cookie alone ties to
// the browser, since there is no session yet, which the callback checks. The callback sends a browser it signs in
// to the query's next, such as /settings, when it is a path on the app, else home.
func (l *Links) signIn(w http.ResponseWriter, r *http.Request) {
	if !l.oauthConfigured() {
		l.login(w, r, "unavailable")
		return
	}
	if next := r.URL.Query().Get("next"); validNext(next) {
		l.nextCookie().Set(w, next)
	} else {
		l.nextCookie().Clear(w) // so an abandoned sign-in's does not send this one there
	}
	l.authorize(w, r, oauth.SignIn, "", signInScope)
}

// validNext reports whether next is a path on the app: // would be another host.
func validNext(next string) bool { return nextPath.MatchString(next) && !strings.HasPrefix(next, "//") }

// next is where a sign-in that works sends the browser: the path its start remembered, else home. The cookie is
// checked again, since a browser can send any.
func (l *Links) next(r *http.Request) string {
	if c, err := r.Cookie(l.nextCookie().Name); err == nil && validNext(c.Value) {
		return c.Value
	}
	return "/"
}

// nextCookie is the github_next cookie, which carries where a sign-in sends the browser to the callback alone, as
// the state cookie carries the state.
func (l *Links) nextCookie() oauth.StateCookie {
	c := l.stateCookie()
	c.Name = "github_next"
	return c
}

// login sends the browser back to sign-in with the outcome of signing in with GitHub, which the page shows.
func (l *Links) login(w http.ResponseWriter, r *http.Request, outcome string) {
	http.Redirect(w, r, l.PublicURL+"/login?github="+outcome, http.StatusFound)
}
