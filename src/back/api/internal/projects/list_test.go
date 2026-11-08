package projects_test

import (
	"fmt"
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func listed(t *testing.T, res *apptest.Response) []project {
	t.Helper()
	var body struct{ Projects []project }
	res.JSON(&body)
	return body.Projects
}

func TestListProjects(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	adaToken := env.LinkGitHub(ada, "ada-gh")
	env.GitHub.AddRepo("octo", "hello").Grant(adaToken, "push")
	hello := env.AddProject(ada, "octo", "hello")
	other := env.GitHub.AddRepo("octo", "other")
	bob := env.SignUp("bob@example.com", "correct horse")
	bobToken := env.LinkGitHub(bob, "bob-gh")
	other.Grant(bobToken, "admin")
	env.AddProject(bob, "octo", "other")

	// Ada sees only what she opened, not bob's project, though GitHub would let her see it.
	other.Grant(adaToken, "pull")
	res := env.Do("GET", "/api/projects", nil, ada)
	res.Expect(http.StatusOK, "")
	if got := listed(t, res); len(got) != 1 || got[0].Name != "hello" || got[0].Permission != "write" {
		t.Fatalf("ada's projects = %+v", got)
	}

	// Opening it adds it, most recent first.
	add(env, ada, "https://github.com/octo/other").Expect(http.StatusOK, "")
	if got := listed(t, env.Do("GET", "/api/projects", nil, ada)); len(got) != 2 || got[0].Name != "other" || got[0].Permission != "read" {
		t.Fatalf("after opening other, ada's projects = %+v", got)
	}

	// Visiting any of a project's pages puts it first again.
	env.Do("GET", fmt.Sprintf("/api/projects/%d/board", hello), nil, ada).Expect(http.StatusOK, "")
	if got := listed(t, env.Do("GET", "/api/projects", nil, ada)); len(got) != 2 || got[0].Name != "hello" {
		t.Fatalf("after visiting hello, ada's projects = %+v", got)
	}

	// Losing access drops it once the cached permission expires.
	other.Grant(adaToken, "")
	expire(t, env)
	if got := listed(t, env.Do("GET", "/api/projects", nil, ada)); len(got) != 1 || got[0].Name != "hello" {
		t.Fatalf("after losing other, ada's projects = %+v", got)
	}
}

func TestListProjectsNeedsLinkedGitHub(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	env.Do("GET", "/api/projects", nil, ada).Expect(http.StatusForbidden, "github_not_linked")
	env.Do("GET", "/api/projects", nil).Expect(http.StatusUnauthorized, "unauthorized")
}

func TestListProjectsAfterRelinking(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	env.AddProject(ada, "octo", "hello")

	// Her token expired, so she links the same account again, as github_not_linked tells her to.
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	if got := listed(t, env.Do("GET", "/api/projects", nil, ada)); len(got) != 1 || got[0].Name != "hello" {
		t.Fatalf("after relinking, ada's projects = %+v, want hello still listed", got)
	}

	// An account that cannot see the repository does not inherit the old one's access.
	env.LinkGitHub(ada, "other-gh")
	if got := listed(t, env.Do("GET", "/api/projects", nil, ada)); len(got) != 0 {
		t.Fatalf("after linking another account, ada's projects = %+v", got)
	}
}
