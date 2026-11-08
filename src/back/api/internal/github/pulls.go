package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
)

// PullRequest is a pull request as automation's triggers read it.
type PullRequest struct {
	Number int    `json:"number"`
	NodeID string `json:"node_id"` // its GraphQL id
	State  string `json:"state"`   // "open" or "closed", merged or not
	Draft  bool   `json:"draft"`
	Head   struct {
		SHA  string `json:"sha"`
		Repo *struct {
			ID int64 `json:"id"`
		} `json:"repo"` // nil once the fork it came from is deleted
	} `json:"head"`
	User User `json:"user"` // its author
}

// ErrPullsChanged reports an open pull request list that changed while PullRequests read its pages, so it may hold a
// pull request twice or miss one.
var ErrPullsChanged = errors.New("github: pull request list changed while it was read")

// PullRequests lists the repository's open pull requests and returns their ETag, or "" when they fill a page or more,
// as Labels does. With etag set, an unchanged first page fails with *Error{Status: 304}. A list that changed between
// its pages fails with ErrPullsChanged.
func (c *Client) PullRequests(ctx context.Context, token string, repoID int64, etag string) ([]PullRequest, string, error) {
	var pulls []PullRequest
	read, err := pages(ctx, c, token, c.repo(repoID)+"/pulls?state=open&per_page=100", etag, func(page []PullRequest) bool {
		pulls = append(pulls, page...)
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
		return nil, "", ErrPullsChanged
	}
	if len(pulls) >= 100 {
		return pulls, "", nil
	}
	return pulls, read[0].etag, nil
}

// PullRequest loads pull request number.
func (c *Client) PullRequest(ctx context.Context, token string, repoID int64, number int) (PullRequest, error) {
	var pr PullRequest
	_, err := c.do(ctx, http.MethodGet, c.repo(repoID)+"/pulls/"+strconv.Itoa(number), token, nil, nil, &pr)
	return pr, err
}

// closingIssues is the ClosingIssues query. The fake GitHub tells operations apart by their names.
// shortcut: a pull request closing more than 50 issues links only its first 50; page the references if one does.
const closingIssues = `query ClosingIssues($ids: [ID!]!) {
  nodes(ids: $ids) {
    ... on PullRequest {
      number
      closingIssuesReferences(first: 50) { nodes { number repository { databaseId } } }
    }
  }
}`

// ClosingIssues returns, by number, the issues each pull request with one of these GraphQL node ids closes, such as
// with "Closes #42". It keeps only issues of the repository repoID: a pull request can close issues elsewhere.
func (c *Client) ClosingIssues(ctx context.Context, token string, repoID int64, nodeIDs []string) (map[int][]int, error) {
	closes := map[int][]int{}
	// GitHub resolves at most 100 node ids in one query.
	for ids := range slices.Chunk(nodeIDs, 100) {
		var out struct {
			Nodes []*struct { // null for a node that is no pull request
				Number                  int `json:"number"`
				ClosingIssuesReferences struct {
					Nodes []struct {
						Number     int `json:"number"`
						Repository struct {
							DatabaseID int64 `json:"databaseId"`
						} `json:"repository"`
					} `json:"nodes"`
				} `json:"closingIssuesReferences"`
			} `json:"nodes"`
		}
		if err := c.graphql(ctx, token, closingIssues, map[string]any{"ids": ids}, &out); err != nil {
			return nil, err
		}
		for _, pr := range out.Nodes {
			if pr == nil {
				continue
			}
			issues := []int{}
			for _, is := range pr.ClosingIssuesReferences.Nodes {
				if is.Repository.DatabaseID == repoID {
					issues = append(issues, is.Number)
				}
			}
			closes[pr.Number] = issues
		}
	}
	return closes, nil
}

// Permission returns login's permission on the repository: "admin", "maintain", "write", "triage", "read" or "none",
// which is also the answer for a login GitHub does not know.
func (c *Client) Permission(ctx context.Context, token string, repoID int64, login string) (string, error) {
	var out struct {
		Permission string `json:"permission"`
	}
	_, err := c.do(ctx, http.MethodGet, c.repo(repoID)+"/collaborators/"+url.PathEscape(login)+"/permission", token, nil, nil, &out)
	if ghErr := (*Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusNotFound {
		return "none", nil
	}
	return out.Permission, err
}

// CreateComment comments body on issue or pull request number.
func (c *Client) CreateComment(ctx context.Context, token string, repoID int64, number int, body string) error {
	_, err := c.do(ctx, http.MethodPost, c.issue(repoID, number)+"/comments", token, nil, map[string]string{"body": body}, nil)
	return err
}

// graphql runs query with vars on GitHub's GraphQL API and decodes its data into out. GraphQL reports most failures
// with 200 and an "errors" list, so any error in the list fails it too.
func (c *Client) graphql(ctx context.Context, token, query string, vars map[string]any, out any) error {
	var res struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if _, err := c.do(ctx, http.MethodPost, c.API+"/graphql", token, nil, map[string]any{"query": query, "variables": vars}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("github: graphql: %s", res.Errors[0].Message)
	}
	return json.Unmarshal(res.Data, out)
}
