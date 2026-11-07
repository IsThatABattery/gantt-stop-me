package projects_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

type project struct {
	ID                      int64
	Owner, Name, Permission string
}

// add posts url to POST /api/projects as session.
func add(env *apptest.Env, session *http.Cookie, url string) *apptest.Response {
	return env.Do("POST", "/api/projects", map[string]string{"url": url}, session)
}

func count(t *testing.T, env *apptest.Env, table string) int {
	t.Helper()
	var n int
	if err := env.DB.QueryRow(context.Background(), `select count(*) from `+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// member returns the permission cached for the user with this email, or "" when none is.
func member(t *testing.T, env *apptest.Env, projectID int64, email string) string {
	t.Helper()
	var perm string
	env.DB.QueryRow(context.Background(), `
		select m.permission from project_members m join users u on u.id = m.user_id
		where m.project_id = $1 and u.email = $2`, projectID, email).Scan(&perm)
	return perm
}

func TestAddProject(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")

	res := add(env, ada, "https://github.com/octo/hello")
	res.Expect(http.StatusCreated, "")
	var p project
	res.JSON(&p)
	if p.ID == 0 || p.Owner != "octo" || p.Name != "hello" || p.Permission != "write" {
		t.Fatalf("project = %+v", p)
	}
	var repoID int64
	var addedBy string
	env.DB.QueryRow(context.Background(), `
		select p.github_repo_id, u.email from projects p join users u on u.id = p.added_by where p.id = $1`, p.ID).Scan(&repoID, &addedBy)
	if repoID != repo.ID || addedBy != "ada@example.com" {
		t.Fatalf("stored repo %d added by %q, want %d by ada", repoID, addedBy, repo.ID)
	}
	if perm := member(t, env, p.ID, "ada@example.com"); perm != "write" {
		t.Fatalf("ada's cached permission = %q, want write", perm)
	}
}

func TestAddProjectAgainReturnsIt(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	repo.AddIssue("First")
	first := env.AddProject(ada, "octo", "hello")

	res := add(env, ada, "https://github.com/octo/hello.git")
	res.Expect(http.StatusOK, "")
	var p project
	res.JSON(&p)
	if p.ID != first {
		t.Fatalf("re-adding returned project %d, want %d", p.ID, first)
	}
	if n := count(t, env, "projects"); n != 1 {
		t.Fatalf("%d projects, want 1", n)
	}
	if n := count(t, env, "columns"); n != 5 {
		t.Fatalf("%d columns, want 5", n)
	}
	if n := count(t, env, "tasks"); n != 1 {
		t.Fatalf("%d tasks, want 1", n)
	}
}

func TestAddProjectURLs(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	token := env.LinkGitHub(ada, "ada-gh")
	env.GitHub.AddRepo("octo", "hello").Grant(token, "push")
	env.GitHub.AddRepo("Octo-Org", "my.repo_name-2").Grant(token, "push")
	long := strings.Repeat("a", 39)
	env.GitHub.AddRepo(long, strings.Repeat("b", 100)).Grant(token, "push")

	accept := map[string]string{
		"https://github.com/octo/hello":                               "octo/hello",
		"https://github.com/octo/hello/":                              "octo/hello",
		"https://github.com/octo/hello.git":                           "octo/hello",
		"https://github.com/octo/hello.git/":                          "octo/hello",
		"https://github.com/OCTO/Hello":                               "octo/hello", // GitHub's spelling is kept
		"https://github.com/Octo-Org/my.repo_name-2":                  "Octo-Org/my.repo_name-2",
		"https://github.com/" + long + "/" + strings.Repeat("b", 100): long + "/" + strings.Repeat("b", 100),
	}
	for url, want := range accept {
		res := add(env, ada, url)
		if res.Status != http.StatusCreated && res.Status != http.StatusOK {
			t.Fatalf("%s: got %d %s, want it added", url, res.Status, res.Body)
		}
		var p project
		res.JSON(&p)
		if got := p.Owner + "/" + p.Name; got != want {
			t.Errorf("%s added %s, want %s", url, got, want)
		}
	}

	reject := []string{
		"",
		"octo/hello",
		"github.com/octo/hello",
		"http://github.com/octo/hello",
		"https://www.github.com/octo/hello",
		"https://gitlab.com/octo/hello",
		"https://github.com/octo",
		"https://github.com/octo/",
		"https://github.com/octo/hello/issues",
		"https://github.com/octo/hello//",
		"https://github.com/octo/hello/.git",
		"https://github.com/octo/hello?tab=readme",
		"https://github.com/octo/hello#readme",
		"https://github.com/octo/hel lo",
		" https://github.com/octo/hello",
		"https://github.com/octo_x/hello",
		"https://github.com/octo/.",
		"https://github.com/octo/..",
		"https://github.com/octo/..git",
		"https://github.com/" + strings.Repeat("a", 40) + "/hello",
		"https://github.com/octo/" + strings.Repeat("b", 101),
	}
	before := len(env.GitHub.Requests())
	for _, url := range reject {
		add(env, ada, url).Expect(http.StatusBadRequest, "invalid_repo_url")
	}
	if got := env.GitHub.Requests()[before:]; len(got) != 0 {
		t.Fatalf("rejected URLs called GitHub: %v", got)
	}
	env.Do("POST", "/api/projects", "{", ada).Expect(http.StatusBadRequest, "bad_request")
}

func TestAddProjectHidesInvisibleRepos(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	token := env.LinkGitHub(ada, "ada-gh")
	env.GitHub.AddRepo("octo", "secret")
	env.GitHub.AddRepo("octo", "void").Grant(token, "none")

	missing := add(env, ada, "https://github.com/octo/nope")
	missing.Expect(http.StatusNotFound, "not_found")
	for _, url := range []string{"https://github.com/octo/secret", "https://github.com/octo/void"} {
		res := add(env, ada, url)
		if res.Status != missing.Status || string(res.Body) != string(missing.Body) {
			t.Errorf("%s: got %d %s, want the same answer as a missing repository", url, res.Status, res.Body)
		}
	}
	if n := count(t, env, "projects"); n != 0 {
		t.Fatalf("%d projects stored, want none", n)
	}
}

func TestAddProjectReadOnly(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	repo.Grant(env.LinkGitHub(bob, "bob-gh"), "pull")

	add(env, bob, "https://github.com/octo/hello").Expect(http.StatusForbidden, "forbidden")
	if n := count(t, env, "projects"); n != 0 {
		t.Fatalf("read-only add stored %d projects", n)
	}
	if labels := repo.Labels(); len(labels) != 0 {
		t.Fatalf("read-only add created labels %v", labels)
	}

	id := env.AddProject(ada, "octo", "hello")
	res := add(env, bob, "https://github.com/octo/hello")
	res.Expect(http.StatusOK, "")
	var p project
	res.JSON(&p)
	if p.ID != id || p.Permission != "read" {
		t.Fatalf("bob got %+v, want project %d with read", p, id)
	}
	if perm := member(t, env, id, "bob@example.com"); perm != "read" {
		t.Fatalf("bob's cached permission = %q, want read", perm)
	}
}

func TestAddProjectPermissions(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	repo := env.GitHub.AddRepo("octo", "hello")
	repo.Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	env.AddProject(ada, "octo", "hello")

	for role, want := range map[string]string{"admin": "admin", "maintain": "write", "push": "write", "triage": "read", "pull": "read"} {
		session := env.SignUp(role+"@example.com", "correct horse")
		repo.Grant(env.LinkGitHub(session, role+"-gh"), role)
		res := add(env, session, "https://github.com/octo/hello")
		res.Expect(http.StatusOK, "")
		var p project
		res.JSON(&p)
		if p.Permission != want {
			t.Errorf("GitHub role %s: permission %q, want %q", role, p.Permission, want)
		}
	}
}

func TestAddProjectNeedsLinkedGitHub(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	add(env, ada, "https://github.com/octo/hello").Expect(http.StatusForbidden, "github_not_linked")
}

func TestAddProjectWhenGitHubIsDown(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.AddRepo("octo", "hello").Grant(env.LinkGitHub(ada, "ada-gh"), "push")
	env.GitHub.Fail("GET /repos/{owner}/{name}", http.StatusInternalServerError)

	add(env, ada, "https://github.com/octo/hello").Expect(http.StatusBadGateway, "github_error")
	if n := count(t, env, "projects"); n != 0 {
		t.Fatalf("%d projects stored, want none", n)
	}
}

func TestAddProjectNeedsSession(t *testing.T) {
	add(apptest.New(t), nil, "https://github.com/octo/hello").Expect(http.StatusUnauthorized, "unauthorized")
}
