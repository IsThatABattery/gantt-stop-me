package projects_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
)

// The access check guards every project route; these tests go through GET /api/projects/{id}/board.

// shared is octo/hello added as a project by ada, who can push to it. Being the first in a fresh schema, its id is 1.
func shared(t *testing.T) (env *apptest.Env, ada *http.Cookie, repo *githubtest.Repo, board string) {
	env = apptest.New(t)
	ada = env.SignUp("ada@example.com", "correct horse")
	repo = env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	return env, ada, repo, fmt.Sprintf("/api/projects/%d/board", env.AddProject(ada, "octo", "hello"))
}

// expire makes every cached permission older than five minutes.
func expire(t *testing.T, env *apptest.Env) {
	t.Helper()
	if _, err := env.DB.Exec(context.Background(), `update project_members set checked_at = now() - interval '5 minutes 1 second'`); err != nil {
		t.Fatal(err)
	}
}

// relinkExpiring relinks ada with OAuth to a token that expires in 8 hours, as GitHub's do, and can push to repo.
// Relinking has the next access check ask GitHub.
func relinkExpiring(env *apptest.Env, ada *http.Cookie, repo *githubtest.Repo) {
	env.GitHub.ExpireTokens(8 * time.Hour)
	repo.Grant(env.LinkGitHubOAuth(ada, "ada-gh"), "push")
}

// GitHub retires a token as it refreshes it, so another request's refresh can beat the access check's read: the check
// asks again with the new token, and the route goes on with it.
func TestAccessAfterAnotherRequestRefreshedTheToken(t *testing.T) {
	env, ada, repo, _ := shared(t)
	relinkExpiring(env, ada, repo)
	refreshed := env.RefreshDuring("GET /repositories/{id}", ada)
	before := len(env.GitHub.Requests())
	// Through assignees, which asks GitHub with the token the check returns: only the new one works.
	env.Do("GET", "/api/projects/1/assignees", nil, ada).Expect(http.StatusOK, "")
	refreshed()
	want := []string{"POST /login/oauth/access_token", "GET /repositories/1", "GET /repositories/1"}
	if got := env.GitHub.Requests()[before:]; len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
		t.Fatalf("GitHub requests = %v, want the other request's refresh, the read it beat, and the read again", got)
	}
	if perm := member(t, env, 1, "ada@example.com"); perm != "write" {
		t.Fatalf("cached permission %q, want write from the check with the new token", perm)
	}
}

func TestAccessWhenGitHubRefusesTheRefreshedTokenToo(t *testing.T) {
	env, ada, repo, board := shared(t)
	relinkExpiring(env, ada, repo)
	refreshed := env.RefreshDuring("GET /repositories/{id}", ada)
	env.GitHub.Fail("GET /repositories/{id}", http.StatusUnauthorized)
	before := len(env.GitHub.Requests())
	env.Do("GET", board, nil, ada).Expect(http.StatusForbidden, "github_not_linked")
	refreshed()
	want := []string{"POST /login/oauth/access_token", "GET /repositories/1", "GET /repositories/1"}
	if got := env.GitHub.Requests()[before:]; !slices.Equal(got, want) {
		t.Fatalf("GitHub requests = %v, want the other request's refresh, the read it beat, and the read again", got)
	}
}

func TestAccessUnknownProject(t *testing.T) {
	env, ada, _, board := shared(t)
	missing := env.Do("GET", "/api/projects/999/board", nil, ada)
	missing.Expect(http.StatusNotFound, "not_found")
	env.Do("GET", "/api/projects/nope/board", nil, ada).Expect(http.StatusNotFound, "not_found")
	env.Do("GET", board, nil, ada).Expect(http.StatusOK, "")
}

func TestAccessHidesInvisibleRepos(t *testing.T) {
	env, ada, _, board := shared(t)
	bob := env.SignUp("bob@example.com", "correct horse")
	env.LinkGitHub(bob, "bob-gh")

	missing := env.Do("GET", "/api/projects/999/board", nil, bob)
	res := env.Do("GET", board, nil, bob)
	if res.Status != http.StatusNotFound || string(res.Body) != string(missing.Body) {
		t.Fatalf("bob got %d %s, want the same 404 as a missing project: %s", res.Status, res.Body, missing.Body)
	}
	if perm := member(t, env, 1, "bob@example.com"); perm != "" {
		t.Fatalf("bob has cached permission %q", perm)
	}
	env.Do("GET", board, nil, ada).Expect(http.StatusOK, "")
}

func TestAccessNeedsLinkedGitHub(t *testing.T) {
	env, _, _, board := shared(t)
	bob := env.SignUp("bob@example.com", "correct horse")
	env.Do("GET", board, nil, bob).Expect(http.StatusForbidden, "github_not_linked")
}

func TestAccessCacheExpires(t *testing.T) {
	env, ada, repo, board := shared(t)
	bob := env.SignUp("bob@example.com", "correct horse")
	bobToken := env.LinkGitHub(bob, "bob-gh")
	repo.Grant(bobToken, "pull")
	env.Do("GET", board, nil, bob).Expect(http.StatusOK, "")

	// Within five minutes, the cached permission stands even after GitHub changes.
	repo.Grant(bobToken, "")
	before := len(env.GitHub.Requests())
	env.Do("GET", board, nil, bob).Expect(http.StatusOK, "")
	if got := env.GitHub.Requests()[before:]; len(got) != 0 {
		t.Fatalf("a cached permission called GitHub: %v", got)
	}

	expire(t, env)
	env.Do("GET", board, nil, bob).Expect(http.StatusNotFound, "not_found")
	if perm := member(t, env, 1, "bob@example.com"); perm != "" {
		t.Fatalf("bob kept cached permission %q after losing access", perm)
	}

	// Gaining access is picked up the same way.
	repo.Grant(bobToken, "push")
	env.Do("GET", board, nil, bob).Expect(http.StatusOK, "")
	if perm := member(t, env, 1, "bob@example.com"); perm != "write" {
		t.Fatalf("bob's cached permission = %q, want write", perm)
	}
	expire(t, env)
	env.Do("GET", board, nil, ada).Expect(http.StatusOK, "")
}

func TestAccessEndsWithTheCredential(t *testing.T) {
	env, ada, _, board := shared(t)
	env.Do("GET", board, nil, ada).Expect(http.StatusOK, "")

	// Relinking to an account without access takes effect at once, despite the fresh cache.
	env.LinkGitHub(ada, "other-gh")
	env.Do("GET", board, nil, ada).Expect(http.StatusNotFound, "not_found")

	env.Do("DELETE", "/api/github", nil, ada).Expect(http.StatusNoContent, "")
	env.Do("GET", board, nil, ada).Expect(http.StatusForbidden, "github_not_linked")
}

func TestAccessWhenGitHubIsDown(t *testing.T) {
	env, ada, _, board := shared(t)
	expire(t, env)
	env.GitHub.Fail("GET /repositories/{id}", http.StatusInternalServerError)
	env.Do("GET", board, nil, ada).Expect(http.StatusBadGateway, "github_error")
	if perm := member(t, env, 1, "ada@example.com"); perm != "write" {
		t.Fatalf("a GitHub outage changed ada's cached permission to %q", perm)
	}
}

func TestAccessTreatsForbiddenAsHidden(t *testing.T) {
	env, ada, _, board := shared(t)
	expire(t, env)
	env.GitHub.Fail("GET /repositories/{id}", http.StatusForbidden)
	env.Do("GET", board, nil, ada).Expect(http.StatusNotFound, "not_found")
}

func TestAccessWithRevokedCredential(t *testing.T) {
	env, ada, _, board := shared(t)
	expire(t, env)
	env.GitHub.Fail("GET /repositories/{id}", http.StatusUnauthorized) // the token expired or was revoked
	env.Do("GET", board, nil, ada).Expect(http.StatusForbidden, "github_not_linked")
}
