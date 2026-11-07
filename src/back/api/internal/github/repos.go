package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Repo is a repository and the calling user's permissions on it.
type Repo struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
	Permissions struct {
		Admin    bool `json:"admin"`
		Maintain bool `json:"maintain"`
		Push     bool `json:"push"`
		Triage   bool `json:"triage"`
		Pull     bool `json:"pull"`
	} `json:"permissions"`
}

// Label is an issue label.
type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Issue is an issue, or a pull request, which GitHub's issue list mixes in.
type Issue struct {
	ID          int64     `json:"id"`
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	State       string    `json:"state"` // "open" or "closed"
	Labels      []Label   `json:"labels"`
	UpdatedAt   time.Time `json:"updated_at"`
	PullRequest *struct{} `json:"pull_request"` // set only on pull requests
}

// repo is the REST URL of the repository owner/name.
func (c *Client) repo(owner, name string) string {
	return c.API + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}

func (c *Client) issue(owner, name string, number int) string {
	return c.repo(owner, name) + "/issues/" + strconv.Itoa(number)
}

// Repo loads the repository owner/name as the token's user sees it.
func (c *Client) Repo(ctx context.Context, token, owner, name string) (Repo, error) {
	var r Repo
	_, err := c.do(ctx, http.MethodGet, c.repo(owner, name), token, nil, nil, &r)
	return r, err
}

// RepoByID loads a repository by its id, which survives renames and transfers.
func (c *Client) RepoByID(ctx context.Context, token string, id int64) (Repo, error) {
	var r Repo
	_, err := c.do(ctx, http.MethodGet, c.API+"/repositories/"+strconv.FormatInt(id, 10), token, nil, nil, &r)
	return r, err
}

// CreateLabel adds a label to the repository. A label that already exists is not an error.
func (c *Client) CreateLabel(ctx context.Context, token, owner, name, label, color string) error {
	_, err := c.do(ctx, http.MethodPost, c.repo(owner, name)+"/labels", token, nil, map[string]string{"name": label, "color": color}, nil)
	// GitHub answers 422 already_exists for a taken name; callers send fixed names and colors, which pass every other check.
	if ghErr := (*Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusUnprocessableEntity {
		return nil
	}
	return err
}

// Issues lists the repository's issues, pull requests included, filtered by query, a page at a time:
// it calls page with each page until page returns false or no pages are left, and returns the first page's ETag.
// With etag set, an unchanged first page fails with *Error{Status: 304}.
func (c *Client) Issues(ctx context.Context, token, owner, name string, query url.Values, etag string, page func([]Issue) bool) (string, error) {
	next := c.repo(owner, name) + "/issues?" + query.Encode()
	var header http.Header
	if etag != "" {
		header = http.Header{"If-None-Match": {etag}}
	}
	var first string
	for i := 0; next != ""; i++ {
		var issues []Issue
		h, err := c.do(ctx, http.MethodGet, next, token, header, nil, &issues)
		if err != nil {
			return "", err
		}
		if i == 0 {
			first, header = h.Get("ETag"), nil
		}
		if !page(issues) {
			break
		}
		next = nextPage(h.Get("Link"))
		// The token goes to this URL, so it must stay on GitHub's API.
		if next != "" && !strings.HasPrefix(next, c.API+"/") {
			return "", fmt.Errorf("github: next page %q is not under %s", next, c.API)
		}
	}
	return first, nil
}

// nextPage returns the rel="next" URL of a Link header, or "" on the last page.
func nextPage(link string) string {
	for part := range strings.SplitSeq(link, ",") {
		target, params, _ := strings.Cut(part, ";")
		if strings.Contains(params, `rel="next"`) {
			return strings.Trim(strings.TrimSpace(target), "<>")
		}
	}
	return ""
}

// Issue loads one issue.
func (c *Client) Issue(ctx context.Context, token, owner, name string, number int) (Issue, error) {
	var is Issue
	_, err := c.do(ctx, http.MethodGet, c.issue(owner, name, number), token, nil, nil, &is)
	return is, err
}

// CreateIssue opens an issue with these labels.
func (c *Client) CreateIssue(ctx context.Context, token, owner, name, title, body string, labels []string) (Issue, error) {
	var is Issue
	in := map[string]any{"title": title, "body": body, "labels": labels}
	_, err := c.do(ctx, http.MethodPost, c.repo(owner, name)+"/issues", token, nil, in, &is)
	return is, err
}

// UpdateIssue sets an issue's fields, any of "title", "body" and "state".
func (c *Client) UpdateIssue(ctx context.Context, token, owner, name string, number int, fields map[string]string) (Issue, error) {
	var is Issue
	_, err := c.do(ctx, http.MethodPatch, c.issue(owner, name, number), token, nil, fields, &is)
	return is, err
}

// AddLabels adds labels to an issue.
func (c *Client) AddLabels(ctx context.Context, token, owner, name string, number int, labels []string) error {
	_, err := c.do(ctx, http.MethodPost, c.issue(owner, name, number)+"/labels", token, nil, map[string][]string{"labels": labels}, nil)
	return err
}

// RemoveLabel takes a label off an issue. A label the issue does not have is not an error.
func (c *Client) RemoveLabel(ctx context.Context, token, owner, name string, number int, label string) error {
	_, err := c.do(ctx, http.MethodDelete, c.issue(owner, name, number)+"/labels/"+url.PathEscape(label), token, nil, nil, nil)
	if ghErr := (*Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}
