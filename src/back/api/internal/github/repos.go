package github

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Repo is a repository and the calling user's permissions on it.
type Repo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
	Owner         struct {
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

// Label is a repository's label, as its issues carry it.
type Label struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Issue is an issue, or a pull request, which GitHub's issue list mixes in.
type Issue struct {
	ID          int64     `json:"id"`
	NodeID      string    `json:"node_id"` // its GraphQL id
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	State       string    `json:"state"` // "open" or "closed"
	Labels      []Label   `json:"labels"`
	UpdatedAt   time.Time `json:"updated_at"`
	PullRequest *struct{} `json:"pull_request"` // set only on pull requests
	User        User      `json:"user"`         // its author
	Assignees   []User    `json:"assignees"`
}

// repo is the REST URL of the repository with this id. Calls go by id, which survives renames and transfers,
// because after a rename GitHub resolves the old owner/name to whichever repository takes it next.
func (c *Client) repo(id int64) string {
	return c.API + "/repositories/" + strconv.FormatInt(id, 10)
}

func (c *Client) issue(repoID int64, number int) string {
	return c.repo(repoID) + "/issues/" + strconv.Itoa(number)
}

// Repo loads the repository owner/name as the token's user sees it.
func (c *Client) Repo(ctx context.Context, token, owner, name string) (Repo, error) {
	var r Repo
	_, err := c.do(ctx, http.MethodGet, c.API+"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name), token, nil, nil, &r)
	return r, err
}

// RepoByID loads a repository by its id.
func (c *Client) RepoByID(ctx context.Context, token string, id int64) (Repo, error) {
	var r Repo
	_, err := c.do(ctx, http.MethodGet, c.repo(id), token, nil, nil, &r)
	return r, err
}

// CreateLabel adds a label to the repository. A label that already exists is not an error.
func (c *Client) CreateLabel(ctx context.Context, token string, repoID int64, label, color string) error {
	_, err := c.do(ctx, http.MethodPost, c.repo(repoID)+"/labels", token, nil, map[string]string{"name": label, "color": color}, nil)
	// GitHub answers 422 already_exists for a taken name; callers send fixed names and colors, which pass every other check.
	if ghErr := (*Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusUnprocessableEntity {
		return nil
	}
	return err
}

// Issues lists the repository's issues, pull requests included, filtered by query, a page at a time:
// it calls page with each page until page returns false or no pages are left, and returns the first page's ETag.
// With etag set, an unchanged first page fails with *Error{Status: 304}.
func (c *Client) Issues(ctx context.Context, token string, repoID int64, query url.Values, etag string, page func([]Issue) bool) (string, error) {
	read, err := pages(ctx, c, token, c.repo(repoID)+"/issues?"+query.Encode(), etag, page)
	if err != nil {
		return "", err
	}
	return read[0].etag, nil
}

// ErrLabelsChanged reports a label list that changed while Labels read its pages, so they may hold a label twice or
// miss one.
var ErrLabelsChanged = errors.New("github: label list changed while it was read")

// Labels lists the repository's labels and returns their ETag, or "" when they fill a page or more: an ETag covers
// only the first page, so a change on a later one would leave it as it was. With etag set, an unchanged first page
// fails with *Error{Status: 304}. A list that changed between its pages fails with ErrLabelsChanged.
func (c *Client) Labels(ctx context.Context, token string, repoID int64, etag string) ([]Label, string, error) {
	labels := []Label{} // not nil, which would send null for a repository with none
	read, err := pages(ctx, c, token, c.repo(repoID)+"/labels?per_page=100", etag, func(page []Label) bool {
		labels = append(labels, page...)
		return true
	})
	if err != nil {
		return nil, "", err
	}
	ok, err := unchanged(ctx, c, token, read)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", ErrLabelsChanged
	}
	if len(labels) >= 100 {
		return labels, "", nil
	}
	return labels, read[0].etag, nil
}

// unchanged re-reads every page of read but the last with its ETag and reports whether all are as they were. An item
// created or deleted mid-walk shifts every item after it across the page boundaries, so one can land on two pages or
// none; that also changes an earlier page, which a free 304 on each of them rules out.
func unchanged(ctx context.Context, c *Client, token string, read []pageRead) (bool, error) {
	for _, p := range read[:len(read)-1] {
		_, err := c.do(ctx, http.MethodGet, p.url, token, http.Header{"If-None-Match": {p.etag}}, nil, nil)
		if err == nil {
			return false, nil
		}
		if ghErr := (*Error)(nil); !errors.As(err, &ghErr) || ghErr.Status != http.StatusNotModified {
			return false, err
		}
	}
	return true, nil
}

// pageRead is a page pages read: its URL and ETag.
type pageRead struct{ url, etag string }

// pages GETs next and each page its Link header names after it, calling page with each until page returns false
// or no pages are left, and returns the pages it read, in order. With etag set, an unchanged first page fails with
// *Error{Status: 304}.
func pages[T any](ctx context.Context, c *Client, token, next, etag string, page func([]T) bool) ([]pageRead, error) {
	var header http.Header
	if etag != "" {
		header = http.Header{"If-None-Match": {etag}}
	}
	var read []pageRead
	for next != "" {
		var items []T
		h, err := c.do(ctx, http.MethodGet, next, token, header, nil, &items)
		if err != nil {
			return nil, err
		}
		header = nil
		read = append(read, pageRead{next, h.Get("ETag")})
		if !page(items) {
			break
		}
		next = nextPage(h.Get("Link"))
		// The token goes to this URL, so it must stay on GitHub's API.
		if next != "" && !strings.HasPrefix(next, c.API+"/") {
			return nil, fmt.Errorf("github: next page %q is not under %s", next, c.API)
		}
	}
	return read, nil
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

// CreateIssue opens an issue with these labels.
func (c *Client) CreateIssue(ctx context.Context, token string, repoID int64, title, body string, labels []string) (Issue, error) {
	var is Issue
	in := map[string]any{"title": title, "body": body, "labels": labels}
	_, err := c.do(ctx, http.MethodPost, c.repo(repoID)+"/issues", token, nil, in, &is)
	return is, err
}

// Issue loads issue number.
func (c *Client) Issue(ctx context.Context, token string, repoID int64, number int) (Issue, error) {
	var is Issue
	_, err := c.do(ctx, http.MethodGet, c.issue(repoID, number), token, nil, nil, &is)
	return is, err
}

// maxWorkflowFile is the most of a workflows file WorkflowFile reads: one byte over automation.ParseWorkflows'
// limit, so the parser still rejects a longer file.
const maxWorkflowFile = 64<<10 + 1

// WorkflowFile returns the repository's .gantt/workflows.yml as it is at commit sha. A file absent at that commit
// fails with *Error{Status: 404}, as does a repository the token cannot see. A file over 64 KiB comes back cut to
// 64 KiB and one byte.
func (c *Client) WorkflowFile(ctx context.Context, token string, repoID int64, sha string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := c.open(ctx, http.MethodGet, c.repo(repoID)+"/contents/.gantt/workflows.yml?ref="+url.QueryEscape(sha), token,
		http.Header{"Accept": {"application/vnd.github.raw+json"}}, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(io.LimitReader(res.Body, maxWorkflowFile))
}

// WorkflowsPath is where a repository keeps its workflows.
const WorkflowsPath = ".gantt/workflows.yml"

// DefaultWorkflows returns the repository's workflows file on its default branch and the file's blob sha, both ""
// when the branch has none.
func (c *Client) DefaultWorkflows(ctx context.Context, token string, repoID int64) (content, sha string, err error) {
	var f struct {
		Content string `json:"content"` // base64, in lines
		SHA     string `json:"sha"`
	}
	_, err = c.do(ctx, http.MethodGet, c.repo(repoID)+"/contents/"+WorkflowsPath, token, nil, nil, &f)
	var ghErr *Error
	if errors.As(err, &ghErr) && ghErr.Status == http.StatusNotFound {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
	if err != nil {
		return "", "", fmt.Errorf("github: workflows file: %w", err)
	}
	return string(data), f.SHA, nil
}

// CreateBranch creates branch name at the default branch's head, and returns the default branch's name. A branch
// that exists already is left as it is, without an error.
func (c *Client) CreateBranch(ctx context.Context, token string, repoID int64, name string) (string, error) {
	repo, err := c.RepoByID(ctx, token, repoID)
	if err != nil {
		return "", err
	}
	var head struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.repo(repoID)+"/git/ref/heads/"+url.PathEscape(repo.DefaultBranch), token, nil, nil, &head); err != nil {
		return "", err
	}
	_, err = c.do(ctx, http.MethodPost, c.repo(repoID)+"/git/refs", token, nil,
		map[string]string{"ref": "refs/heads/" + name, "sha": head.Object.SHA}, nil)
	// GitHub answers every validation failure with 422, such as a ruleset or a clash with a branch named feature.
	var ghErr *Error
	if errors.As(err, &ghErr) && ghErr.Status == http.StatusUnprocessableEntity && ghErr.Message == "Reference already exists" {
		err = nil
	}
	return repo.DefaultBranch, err
}

// CreatePullRequest opens a pull request from branch head into base and returns its number and web URL.
func (c *Client) CreatePullRequest(ctx context.Context, token string, repoID int64, title, body, head, base string) (int, string, error) {
	var out struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	_, err := c.do(ctx, http.MethodPost, c.repo(repoID)+"/pulls", token, nil,
		map[string]string{"title": title, "body": body, "head": head, "base": base}, &out)
	return out.Number, out.HTMLURL, err
}

// PutWorkflows commits content as the workflows file on branch with message, replacing the blob sha, or creating the
// file when sha is "". A sha that is no longer the file's fails with *Error{Status: 409}, and creating a file that
// exists with *Error{Status: 422}. It returns the new blob's sha.
func (c *Client) PutWorkflows(ctx context.Context, token string, repoID int64, branch, content, sha, message string) (string, error) {
	in := map[string]string{"message": message, "content": base64.StdEncoding.EncodeToString([]byte(content)), "branch": branch}
	if sha != "" {
		in["sha"] = sha
	}
	var out struct {
		Content struct {
			SHA string `json:"sha"`
		} `json:"content"`
	}
	_, err := c.do(ctx, http.MethodPut, c.repo(repoID)+"/contents/"+WorkflowsPath, token, nil, in, &out)
	return out.Content.SHA, err
}

// Tarball streams the repository's source at commit sha: GitHub's .tar.gz, with everything in one owner-repo-sha/
// folder after a pax global header. The caller closes it. It has no time limit of its own, since a large repository
// takes longer than any one limit to download: ctx bounds it. GitHub redirects it to codeload.github.com, and the
// HTTP client sends the token only to api.github.com, as Go's does by dropping Authorization on a redirect to
// another host.
func (c *Client) Tarball(ctx context.Context, token string, repoID int64, sha string) (io.ReadCloser, error) {
	res, err := c.open(ctx, http.MethodGet, c.repo(repoID)+"/tarball/"+url.PathEscape(sha), token, nil, nil)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// UpdateIssue sets an issue's fields, any of "title", "body" and "state".
func (c *Client) UpdateIssue(ctx context.Context, token string, repoID int64, number int, fields map[string]any) (Issue, error) {
	var is Issue
	_, err := c.do(ctx, http.MethodPatch, c.issue(repoID, number), token, nil, fields, &is)
	return is, err
}

// Assignable lists the users the repository's issues can be assigned to.
func (c *Client) Assignable(ctx context.Context, token string, repoID int64) ([]User, error) {
	users := []User{}
	_, err := pages(ctx, c, token, c.repo(repoID)+"/assignees?per_page=100", "", func(page []User) bool {
		users = append(users, page...)
		return true
	})
	return users, err
}

// AddAssignees assigns logins to an issue and returns the issue as it is now.
func (c *Client) AddAssignees(ctx context.Context, token string, repoID int64, number int, logins []string) (Issue, error) {
	var is Issue
	_, err := c.do(ctx, http.MethodPost, c.issue(repoID, number)+"/assignees", token, nil, map[string][]string{"assignees": logins}, &is)
	return is, err
}

// AddLabels adds labels to an issue and returns every label it has now.
func (c *Client) AddLabels(ctx context.Context, token string, repoID int64, number int, labels []string) ([]Label, error) {
	var all []Label
	_, err := c.do(ctx, http.MethodPost, c.issue(repoID, number)+"/labels", token, nil, map[string][]string{"labels": labels}, &all)
	return all, err
}

// RemoveLabel takes a label off an issue. A label the issue does not have is not an error.
func (c *Client) RemoveLabel(ctx context.Context, token string, repoID int64, number int, label string) error {
	_, err := c.do(ctx, http.MethodDelete, c.issue(repoID, number)+"/labels/"+url.PathEscape(label), token, nil, nil, nil)
	if ghErr := (*Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}
