// Package githubtest fakes the parts of GitHub the API calls, for tests.
package githubtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// Fake serves GitHub's REST API and its OAuth endpoints from one test server, so a Client
// can point both its API and Web base URLs at Server.URL.
type Fake struct {
	Server       *httptest.Server
	ClientID     string // the only OAuth App credentials the token exchange accepts
	ClientSecret string

	mux   *http.ServeMux
	mu    sync.Mutex
	users map[string]user   // by token
	codes map[string]string // OAuth code -> token
}

type user struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// New starts a Fake, closed when the test ends.
func New(t testing.TB) *Fake {
	f := &Fake{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		mux:          http.NewServeMux(),
		users:        map[string]user{},
		codes:        map[string]string{},
	}
	f.mux.HandleFunc("GET /user", f.user)
	f.mux.HandleFunc("POST /login/oauth/access_token", f.accessToken)
	f.Server = httptest.NewServer(f.mux)
	t.Cleanup(f.Server.Close)
	return f
}

// AddUser makes token authenticate as the GitHub account id and login.
func (f *Fake) AddUser(token string, id int64, login string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[token] = user{id, login}
}

// AddCode makes code a valid, single-use OAuth code that exchanges for token.
func (f *Fake) AddCode(code, token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes[code] = token
}

// auth returns the user the request's bearer token belongs to. On failure it answers 401 like GitHub.
func (f *Fake) auth(w http.ResponseWriter, r *http.Request) (user, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	f.mu.Lock()
	u, found := f.users[token]
	f.mu.Unlock()
	if !ok || !found {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
		return user{}, false
	}
	return u, true
}

func (f *Fake) user(w http.ResponseWriter, r *http.Request) {
	if u, ok := f.auth(w, r); ok {
		writeJSON(w, http.StatusOK, u)
	}
}

func (f *Fake) accessToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Code         string `json:"code"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	f.mu.Lock()
	token, found := f.codes[in.Code]
	delete(f.codes, in.Code)
	f.mu.Unlock()

	out := map[string]string{"access_token": token, "token_type": "bearer", "scope": "repo"}
	switch {
	case in.ClientID != f.ClientID || in.ClientSecret != f.ClientSecret:
		out = map[string]string{"error": "incorrect_client_credentials"}
	case !found:
		out = map[string]string{"error": "bad_verification_code"}
	}
	// Like GitHub: JSON only on request, otherwise form encoding, and 200 even for errors.
	if r.Header.Get("Accept") != "application/json" {
		form := url.Values{}
		for k, v := range out {
			form.Set(k, v)
		}
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		w.Write([]byte(form.Encode()))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
