package github_test

import (
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/github/githubtest"
)

// fake is a fake GitHub with octo/hello, which token can push to.
func fake(t *testing.T) (*githubtest.Fake, *githubtest.Repo, *github.Client) {
	f := githubtest.New(t)
	f.AddUser("token", 1, "ada")
	repo := f.AddRepo("octo", "hello")
	repo.Grant("token", "push")
	return f, repo, &github.Client{API: f.Server.URL, HTTP: f.Server.Client()}
}

func TestPullRequestsETag(t *testing.T) {
	_, repo, gh := fake(t)
	repo.AddPull("One", githubtest.Pull{Author: "ada", HeadSHA: "a1", HeadRepo: repo.ID})
	pulls, etag, err := gh.PullRequests(t.Context(), "token", repo.ID, "")
	if err != nil || len(pulls) != 1 || etag == "" {
		t.Fatalf("PullRequests = %d pulls, ETag %q, %v", len(pulls), etag, err)
	}
	if pr := pulls[0]; pr.Number != 1 || pr.User.Login != "ada" || pr.Head.SHA != "a1" || pr.Head.Repo == nil || pr.Head.Repo.ID != repo.ID {
		t.Fatalf("pull request = %+v", pr)
	}
	var ghErr *github.Error
	if _, _, err := gh.PullRequests(t.Context(), "token", repo.ID, etag); !errors.As(err, &ghErr) || ghErr.Status != http.StatusNotModified {
		t.Fatalf("PullRequests with an unchanged list's ETag: err = %v, want HTTP 304", err)
	}

	// A list of 100 or more fills a page, which its ETag cannot cover.
	for range 99 {
		repo.AddPull("More", githubtest.Pull{Author: "ada"})
	}
	pulls, etag, err = gh.PullRequests(t.Context(), "token", repo.ID, etag)
	if err != nil || len(pulls) != 100 || etag != "" {
		t.Fatalf("PullRequests of 100 = %d pulls, ETag %q, %v; want no ETag", len(pulls), etag, err)
	}
}

func TestClosingIssuesKeepsTheRepositorysOwn(t *testing.T) {
	f, repo, gh := fake(t)
	other := f.AddRepo("octo", "other")
	repo.AddIssue("One")
	n := repo.AddPull("Fix", githubtest.Pull{Author: "ada", Closes: []githubtest.Ref{{RepoID: repo.ID, Number: 1}, {RepoID: other.ID, Number: 1}}})
	pulls, _, err := gh.PullRequests(t.Context(), "token", repo.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	closes, err := gh.ClosingIssues(t.Context(), "token", repo.ID, []string{pulls[0].NodeID})
	if want := map[int][]int{n: {1}}; err != nil || !maps.EqualFunc(closes, want, slices.Equal) {
		t.Fatalf("ClosingIssues = %v, %v, want %v", closes, err, want)
	}
}

func TestGraphQLErrorsFailWith200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data": {"nodes": [null]}, "errors": [{"type": "NOT_FOUND", "message": "Could not resolve to a node"}]}`))
	}))
	defer srv.Close()
	gh := &github.Client{API: srv.URL, HTTP: srv.Client()}
	if _, err := gh.ClosingIssues(t.Context(), "token", 1, []string{"PR_1"}); err == nil {
		t.Fatal("ClosingIssues ignored GraphQL errors")
	}
}

func TestPermission(t *testing.T) {
	f, repo, gh := fake(t)
	f.AddUser("bob-token", 2, "bob")
	repo.Grant("bob-token", "triage")
	f.AddUser("eve-token", 3, "eve") // no role on the repository
	for login, want := range map[string]string{"ada": "write", "bob": "read", "eve": "none", "nobody": "none"} {
		if got, err := gh.Permission(t.Context(), "token", repo.ID, login); err != nil || got != want {
			t.Errorf("Permission(%s) = %q, %v, want %q", login, got, err, want)
		}
	}
	f.Fail("GET /repositories/{id}/collaborators/{login}/permission", http.StatusBadGateway)
	var ghErr *github.Error
	if _, err := gh.Permission(t.Context(), "token", repo.ID, "ada"); !errors.As(err, &ghErr) || ghErr.Status != http.StatusBadGateway {
		t.Fatalf("Permission with GitHub failing: err = %v", err)
	}
}

func TestConvertToDraft(t *testing.T) {
	f, repo, gh := fake(t)
	n := repo.AddPull("Fix", githubtest.Pull{Author: "ada", HeadSHA: "a1", HeadRepo: repo.ID})
	pulls, _, err := gh.PullRequests(t.Context(), "token", repo.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	f.AddUser("bob-token", 2, "bob")
	repo.Grant("bob-token", "triage")
	if err := gh.ConvertToDraft(t.Context(), "bob-token", pulls[0].NodeID); err == nil {
		t.Fatal("ConvertToDraft without push access succeeded")
	}
	if err := gh.ConvertToDraft(t.Context(), "token", "PR_404"); err == nil {
		t.Fatal("ConvertToDraft of an unknown node succeeded")
	}
	if repo.Issue(n).Pull.Draft {
		t.Fatal("a refused ConvertToDraft made a draft")
	}
	if err := gh.ConvertToDraft(t.Context(), "token", pulls[0].NodeID); err != nil || !repo.Issue(n).Pull.Draft {
		t.Fatalf("ConvertToDraft = %v, draft %v", err, repo.Issue(n).Pull.Draft)
	}
	f.Fail("POST /graphql", http.StatusBadGateway)
	if err := gh.ConvertToDraft(t.Context(), "token", pulls[0].NodeID); !isStatus(err, http.StatusBadGateway) {
		t.Fatalf("ConvertToDraft with GitHub failing: err = %v", err)
	}
}
