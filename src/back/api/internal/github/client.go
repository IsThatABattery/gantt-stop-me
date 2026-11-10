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

// Client calls GitHub's REST API, and knows where its OAuth endpoints are.
type Client struct {
	API  string // REST API base URL, https://api.github.com
	Web  string // site base URL, https://github.com, for the OAuth endpoints
	HTTP *http.Client
}

// Error is a non-2xx answer from GitHub.
type Error struct {
	Status  int
	Message string // GitHub's message, such as why it will not merge, or "" when its answer had none
}

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
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// User loads the account token belongs to.
func (c *Client) User(ctx context.Context, token string) (User, error) {
	var u User
	_, err := c.do(ctx, http.MethodGet, c.API+"/user", token, nil, nil, &u)
	return u, err
}

// Email is one of a GitHub account's email addresses.
type Email struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// Emails lists the email addresses of the account token belongs to, which needs the user:email scope.
// shortcut: the first 100 only, where an account's primary one is unless it has over 100.
func (c *Client) Emails(ctx context.Context, token string) ([]Email, error) {
	var emails []Email
	_, err := c.do(ctx, http.MethodGet, c.API+"/user/emails?per_page=100", token, nil, nil, &emails)
	return emails, err
}

// do sends one request to the absolute URL target and returns the response headers, for ETag and Link.
// token, if set, is sent as a bearer token; header adds or overrides request headers, such as If-None-Match;
// in, if not nil, is sent as JSON; out, if not nil, receives the JSON answer.
// A non-2xx answer returns *Error.
func (c *Client) do(ctx context.Context, method, target, token string, header http.Header, in, out any) (http.Header, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := c.open(ctx, method, target, token, header, in)
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
func (c *Client) open(ctx context.Context, method, target, token string, header http.Header, in any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
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
	var e struct {
		Message string `json:"message"`
	}
	json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&e)
	status := res.StatusCode
	// GitHub answers some rate limits with 403; report them as the 429 it uses for others, not as lost access.
	if status == http.StatusForbidden && (res.Header.Get("X-RateLimit-Remaining") == "0" || res.Header.Get("Retry-After") != "" ||
		strings.Contains(strings.ToLower(e.Message), "rate limit")) {
		status = http.StatusTooManyRequests
	}
	return nil, &Error{Status: status, Message: e.Message}
}
