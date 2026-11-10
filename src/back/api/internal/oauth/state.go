// Package oauth holds the pieces of OAuth that are the same for every provider: the state a flow carries in a
// cookie from its start to its callback, the token endpoint client, and what refreshing expiring tokens needs.
package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Kind is the flow a state starts and its callback finishes.
type Kind string

const (
	// Link links an account to the signed-in user. Its state is bound to their session.
	Link Kind = "link"
	// SignIn signs in with an account. There is no session yet, so the cookie alone binds its state to the browser.
	SignIn Kind = "signin"
)

// StateTTL is how long a flow has from its start to its callback.
const StateTTL = 10 * time.Minute

// StateCookie is the cookie that carries a flow's state from its start to its callback, the only path it is sent
// to. The provider sends the state back in the callback's query, so a callback whose state is not the cookie came
// from no flow this browser started.
type StateCookie struct {
	Name   string // such as github_state
	Path   string // the callback's path
	Secure bool   // sent over https only, as when the app's URL is https
}

// Issue mints a state for a flow of kind, sets the cookie to it for StateTTL and returns it, for the authorize URL.
// session, if not "", binds the state to that session token, so only that session finishes the flow; else the
// cookie alone binds it to the browser.
func (c StateCookie) Issue(w http.ResponseWriter, kind Kind, session string) string {
	nonce := rand.Text()
	state := string(kind) + "." + nonce
	if session != "" {
		state += "." + sign(session, nonce)
	}
	c.Set(w, state)
	return state
}

// Set sets the cookie to value for StateTTL. Issue sets a state cookie; a cookie with the same attributes and another
// name carries anything else a flow brings to its callback.
func (c StateCookie) Set(w http.ResponseWriter, value string) {
	http.SetCookie(w, c.cookie(value, int(StateTTL.Seconds())))
}

// Clear deletes the cookie.
func (c StateCookie) Clear(w http.ResponseWriter) { http.SetCookie(w, c.cookie("", -1)) }

// Verify checks that the callback's state is the cookie, and that Issue minted it for a flow of kind and for
// session, the session token it was bound to, or "" for one bound to the browser alone.
func (c StateCookie) Verify(r *http.Request, kind Kind, session string) error {
	state := r.URL.Query().Get("state")
	ck, err := r.Cookie(c.Name)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(ck.Value), []byte(state)) != 1 {
		return fmt.Errorf("state does not match the %s cookie", c.Name)
	}
	k, rest, _ := strings.Cut(state, ".")
	nonce, mac, bound := strings.Cut(rest, ".")
	if Kind(k) != kind || nonce == "" || bound && session == "" {
		return fmt.Errorf("state is not for a %s flow", kind)
	}
	if session != "" && (!bound || !hmac.Equal([]byte(mac), []byte(sign(session, nonce)))) {
		return errors.New("state is not tied to this session")
	}
	return nil
}

// Flow is the kind of flow the callback's state says it is for or, when the callback has none, the cookie's, so a
// callback that lost its state still fails as its flow does. Only Verify shows Issue minted either.
func (c StateCookie) Flow(r *http.Request) Kind {
	state := r.URL.Query().Get("state")
	if ck, err := r.Cookie(c.Name); state == "" && err == nil {
		state = ck.Value
	}
	return KindOf(state)
}

// KindOf is the kind of flow state says it is for, or "" for none this package knows. Anyone can send a callback
// any state, so only Verify shows Issue minted it.
func KindOf(state string) Kind {
	switch k, _, _ := strings.Cut(state, "."); Kind(k) {
	case Link, SignIn:
		return Kind(k)
	}
	return ""
}

// sign proves a state came from this session without putting the session token or its hash in a URL.
func sign(session, nonce string) string {
	m := hmac.New(sha256.New, []byte(session))
	m.Write([]byte(nonce))
	return hex.EncodeToString(m.Sum(nil))
}

// cookie builds the state cookie; maxAge < 0 deletes it.
func (c StateCookie) cookie(state string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     c.Name,
		Value:    state,
		Path:     c.Path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}
