package github

import (
	"crypto/hmac"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"gantt-stop-me/api/internal/accounts"
)

// callback finishes linking with OAuth and sends the browser back to Settings with the outcome.
func (l *Links) callback(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, l.cookie("", -1))
	outcome := "linked"
	if err := l.finishOAuth(r); err != nil {
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		outcome = "error"
	}
	http.Redirect(w, r, l.PublicURL+"/settings?github="+outcome, http.StatusFound)
}

// finishOAuth checks state, exchanges the code and saves the link. It saves nothing on failure.
func (l *Links) finishOAuth(r *http.Request) error {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		return fmt.Errorf("authorization refused: %q", e)
	}
	c, err := r.Cookie(stateCookie)
	if err != nil || q.Get("state") == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(q.Get("state"))) != 1 {
		return errors.New("state does not match the github_state cookie")
	}
	session, _ := r.Cookie(accounts.SessionCookie) // Require guarantees it
	nonce, mac, _ := strings.Cut(q.Get("state"), ".")
	if !hmac.Equal([]byte(mac), []byte(stateMAC(session.Value, nonce))) {
		return errors.New("state is not tied to this session")
	}
	token, err := l.Client.ExchangeCode(r.Context(), l.ClientID, l.ClientSecret, q.Get("code"), l.PublicURL+callbackPath)
	if err != nil {
		return err
	}
	u, err := l.Client.User(r.Context(), token)
	if err != nil {
		return err
	}
	return l.save(r.Context(), accounts.UserID(r.Context()), u, token, "oauth")
}
