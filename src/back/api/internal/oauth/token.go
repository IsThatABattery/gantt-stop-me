package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Endpoint is an OAuth app's token endpoint, which trades the app's authorization codes and refresh tokens for user
// access tokens.
type Endpoint struct {
	TokenURL     string // such as https://github.com/login/oauth/access_token
	ClientID     string
	ClientSecret string
	HTTP         *http.Client
}

// Token is a user access token from a token endpoint.
type Token struct {
	AccessToken    string
	RefreshToken   string    // "" when AccessToken does not expire
	Expires        time.Time // when AccessToken expires; zero when it does not
	RefreshExpires time.Time // when RefreshToken expires; zero without one
}

// Error is a token endpoint refusing a code or a refresh token, such as with invalid_grant, or GitHub's
// bad_verification_code or bad_refresh_token. Asking again gets the same answer.
type Error struct {
	Code        string
	Description string // the endpoint's error_description, or ""
}

// Error quotes the endpoint's code and description, so logging one cannot forge a log line.
func (e *Error) Error() string {
	if e.Description == "" {
		return fmt.Sprintf("oauth: the token endpoint refused: %q", e.Code)
	}
	return fmt.Sprintf("oauth: the token endpoint refused: %q: %q", e.Code, e.Description)
}

// StatusError is a token endpoint's non-2xx answer that is no refusal, such as its 5xx or 429, or GitHub's 404 for
// a client id it does not know.
type StatusError struct {
	Status int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("oauth: the token endpoint answered HTTP %d", e.Status)
}

// Exchange trades an authorization code, issued for redirectURI, for a user access token.
func (e *Endpoint) Exchange(ctx context.Context, code, redirectURI string) (Token, error) {
	return e.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI}})
}

// Refresh trades a refresh token for a new user access token, with the same scopes, and a new refresh token where
// the endpoint rotates them, as GitHub's does: once it answers, the old access token and refresh token no longer work.
func (e *Endpoint) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	return e.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}})
}

// timeout bounds each request to a token endpoint.
const timeout = 15 * time.Second

// token posts form, with the app's credentials, to the endpoint. A refusal returns *Error, with HTTP 200 as
// GitHub's, or 400 or 401 as RFC 6749's; any other non-2xx answer returns *StatusError.
func (e *Endpoint) token(ctx context.Context, form url.Values) (Token, error) {
	form.Set("client_id", e.ClientID)
	form.Set("client_secret", e.ClientSecret)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json") // GitHub's answers JSON only when asked to
	now := time.Now()                            // before the endpoint answers, so the expiry is never later than its
	res, err := e.HTTP.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer res.Body.Close()
	var out struct {
		AccessToken           string `json:"access_token"`
		RefreshToken          string `json:"refresh_token"`
		ExpiresIn             int64  `json:"expires_in"`
		RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
		Error                 string `json:"error"`
		ErrorDescription      string `json:"error_description"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&out)
	ok := res.StatusCode >= 200 && res.StatusCode <= 299
	switch {
	case out.Error != "" && (ok || res.StatusCode == http.StatusBadRequest || res.StatusCode == http.StatusUnauthorized):
		return Token{}, &Error{Code: out.Error, Description: out.ErrorDescription}
	case !ok:
		return Token{}, &StatusError{Status: res.StatusCode}
	case decodeErr != nil:
		return Token{}, fmt.Errorf("oauth: decode the token endpoint's answer: %w", decodeErr)
	case out.AccessToken == "":
		return Token{}, errors.New("oauth: the token endpoint answered no access token")
	}
	tok := Token{AccessToken: out.AccessToken, RefreshToken: out.RefreshToken}
	if out.ExpiresIn > 0 {
		tok.Expires = now.Add(time.Duration(out.ExpiresIn) * time.Second)
	}
	if out.RefreshTokenExpiresIn > 0 {
		tok.RefreshExpires = now.Add(time.Duration(out.RefreshTokenExpiresIn) * time.Second)
	}
	return tok, nil
}
