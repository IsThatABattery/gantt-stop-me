package github

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/oauth"
)

// token links the GitHub account a personal access token belongs to.
func (l *Links) token(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	token := strings.TrimSpace(in.Token)
	if token == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Enter a personal access token.")
		return
	}
	// Go will not send a control character in a header, and that error would read as GitHub being down.
	if strings.ContainsFunc(token, unicode.IsControl) {
		httpx.Error(w, http.StatusBadRequest, "invalid_token", "GitHub did not accept this token.")
		return
	}
	// A token link would replace the OAuth link a user without a password signs in with.
	if !l.passwordSet(w, r, "Set a password before linking a token, so you can still sign in.") {
		return
	}
	u, err := l.Client.User(r.Context(), token)
	if ghErr := (*Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusUnauthorized {
		httpx.Error(w, http.StatusBadRequest, "invalid_token", "GitHub did not accept this token.")
		return
	}
	if err != nil {
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		httpx.Error(w, http.StatusBadGateway, "github_unavailable", "GitHub could not be reached. Try again.")
		return
	}
	if err := l.save(r.Context(), l.DB, accounts.UserID(r.Context()), u, oauth.Token{AccessToken: token}, "pat"); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"login": u.Login, "kind": "pat"})
}
