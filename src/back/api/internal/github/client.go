package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gantt-stop-me/api/internal/httpx"
)

// Client calls GitHub's REST API and its OAuth endpoints.
type Client struct {
	API  string // REST API base URL, https://api.github.com
	Web  string // site base URL for OAuth, https://github.com
	HTTP *http.Client
}

// Error is a non-2xx answer from GitHub.
type Error struct{ Status int }

func (e *Error) Error() string { return fmt.Sprintf("github: HTTP %d", e.Status) }

// Fail answers a request whose GitHub call failed: 403 github_not_linked when GitHub no longer accepts the
// credential (401), 403 forbidden when GitHub refused it with 403 or 404, else 502 github_error.
func Fail(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	status := 0
	if ghErr := (*Error)(nil); errors.As(err, &ghErr) {
		status = ghErr.Status
	}
	switch status {
	case http.StatusUnauthorized:
		httpx.Error(w, http.StatusForbidden, "github_not_linked", "GitHub no longer accepts your linked credential. Link your GitHub account again in Settings.")
	case http.StatusForbidden, http.StatusNotFound:
		httpx.Error(w, http.StatusForbidden, "forbidden", "GitHub refused this. Your GitHub account may have lost access to the repository.")
	default:
		httpx.Error(w, http.StatusBadGateway, "github_error", "GitHub could not be reached or did not accept the change. Try again.")
	}
}

// User is a GitHub account.
type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// User loads the account token belongs to.
func (c *Client) User(ctx context.Context, token string) (User, error) {
	var u User
	_, err := c.do(ctx, http.MethodGet, c.API+"/user", token, nil, nil, &u)
	return u, err
}

// ExchangeCode trades an OAuth authorization code for an access token.
func (c *Client) ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (string, error) {
	in := map[string]string{"client_id": clientID, "client_secret": clientSecret, "code": code, "redirect_uri": redirectURI}
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	// This endpoint answers JSON only when asked for application/json, and reports a bad code with 200.
	_, err := c.do(ctx, http.MethodPost, c.Web+"/login/oauth/access_token", "", http.Header{"Accept": {"application/json"}}, in, &out)
	if err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("github: code exchange failed: %q", out.Error)
	}
	return out.AccessToken, nil
}

// do sends one request to the absolute url and returns the response headers, for ETag and Link.
// token, if set, is sent as a bearer token; header adds or overrides request headers, such as If-None-Match;
// in, if not nil, is sent as JSON; out, if not nil, receives the JSON answer.
// A non-2xx answer returns *Error.
func (c *Client) do(ctx context.Context, method, url, token string, header http.Header, in, out any) (http.Header, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := c.open(ctx, method, url, token, header, in)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return res.Header, fmt.Errorf("github: decode %s %s: %w", method, res.Request.URL.Path, err)
		}
	}
	return res.Header, nil
}

// timeout bounds each call but Tarball, whose download only the caller's context bounds.
const timeout = 15 * time.Second

// open sends one request as do does, with no time limit of its own, and returns the 2xx answer, whose body the
// caller closes. A non-2xx answer returns *Error.
func (c *Client) open(ctx context.Context, method, url, token string, header http.Header, in any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		return res, nil
	}
	defer res.Body.Close()
	status := res.StatusCode
	if status == http.StatusForbidden {
		var e struct {
			Message string `json:"message"`
		}
		json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&e)
		// GitHub answers some rate limits with 403; report them as the 429 it uses for others, not as lost access.
		if res.Header.Get("X-RateLimit-Remaining") == "0" || res.Header.Get("Retry-After") != "" ||
			strings.Contains(strings.ToLower(e.Message), "rate limit") {
			status = http.StatusTooManyRequests
		}
	}
	return nil, &Error{Status: status}
}
