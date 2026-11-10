package github_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"net/http"
	"testing"
	"time"

	"gantt-stop-me/api/internal/apptest"
)

func TestLinkToken(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.AddUser("ghp_personal", 7, "ada-gh")

	res := env.Do("POST", "/api/github/token", map[string]string{"token": " ghp_personal\n"}, session)
	res.Expect(http.StatusOK, "")
	var out struct{ Login, Kind string }
	res.JSON(&out)
	if out.Login != "ada-gh" || out.Kind != "pat" {
		t.Fatalf("token link returned %+v", out)
	}
	if bytes.Contains(res.Body, []byte("ghp_personal")) {
		t.Fatal("response echoes the token")
	}
	links := stored(t, env)
	if len(links) != 1 || links[0] != (storedLink{GitHubID: 7, Login: "ada-gh", Kind: "pat", Token: "ghp_personal"}) {
		t.Fatalf("github_links = %+v, want one pat link to ada-gh", links)
	}
}

func TestLinkTokenReplacesOldLink(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	env.LinkGitHub(session, "old-gh")
	env.LinkGitHub(bob, "bob-gh")
	addMembership(t, env, session)
	addMembership(t, env, bob)
	token := env.LinkGitHub(session, "new-gh")

	links := stored(t, env)
	if len(links) != 2 || links[0].Login != "new-gh" || links[0].Token != token {
		t.Fatalf("github_links = %+v, want the new link and bob's", links)
	}
	// Access was checked with the old credential, so it must not outlive it, but the project stays opened.
	var fresh int
	if err := env.DB.QueryRow(context.Background(), `select count(*) from project_members where user_id = $1 and checked_at > now() - interval '5 minutes'`,
		userID(t, env, session)).Scan(&fresh); err != nil {
		t.Fatal(err)
	}
	if n := memberships(t, env, session); n != 1 || fresh != 0 {
		t.Fatalf("relinking left %d project memberships, %d still fresh; want 1, stale", n, fresh)
	}
	if n := memberships(t, env, bob); n != 1 {
		t.Fatalf("bob has %d project memberships, want his 1 untouched", n)
	}
}

func TestLinkTokenReplacesAnExpiringLink(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.ExpireTokens(8 * time.Hour)
	env.LinkGitHubOAuth(session, "ada-gh")
	token := env.LinkGitHub(session, "ada-gh")

	if links := stored(t, env); len(links) != 1 || links[0] != (storedLink{GitHubID: links[0].GitHubID, Login: "ada-gh", Kind: "pat", Token: token}) {
		t.Fatalf("github_links = %+v, want the pat link, with no refresh token or expiry", links)
	}
}

func TestLinkTokenRejectsBadTokens(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	token := env.LinkGitHub(session, "octocat")

	for _, body := range []any{map[string]string{"token": ""}, map[string]string{"token": " \n"}, "{"} {
		env.Do("POST", "/api/github/token", body, session).Expect(http.StatusBadRequest, "bad_request")
	}
	env.Do("POST", "/api/github/token", map[string]string{"token": "ghp_unknown"}, session).
		Expect(http.StatusBadRequest, "invalid_token")
	for _, tok := range []string{"ghp_abc\ndef", "ghp_abc\x00def", "ghp_abc\x7fdef"} {
		env.Do("POST", "/api/github/token", map[string]string{"token": tok}, session).
			Expect(http.StatusBadRequest, "invalid_token")
	}

	if links := stored(t, env); len(links) != 1 || links[0].Login != "octocat" || links[0].Token != token {
		t.Fatalf("github_links = %+v, want the earlier link untouched", links)
	}
}

func TestLinkTokenWhenGitHubIsDown(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.AddUser("ghp_personal", 7, "ada-gh")
	env.GitHub.Server.Close()

	env.Do("POST", "/api/github/token", map[string]string{"token": "ghp_personal"}, session).
		Expect(http.StatusBadGateway, "github_unavailable")
	if links := stored(t, env); len(links) != 0 {
		t.Fatalf("github_links = %+v, want none", links)
	}
}

// A user made by signing in with GitHub has no password and signs in only with that OAuth link, which a token link
// would replace.
func TestLinkTokenNeedsAPassword(t *testing.T) {
	env := apptest.New(t)
	session, token := env.SignInWithGitHub("octocat", "octo@example.com")
	pat := "ghp_" + rand.Text()
	env.GitHub.AddUser(pat, apptest.GitHubID("octocat"), "octocat")

	env.Do("POST", "/api/github/token", map[string]string{"token": pat}, session).Expect(http.StatusConflict, "password_required")
	if links := stored(t, env); len(links) != 1 || links[0].Kind != "oauth" || links[0].Token != token {
		t.Fatalf("github_links = %+v, want the OAuth link untouched", links)
	}
}

func TestLinkTokenNeedsSession(t *testing.T) {
	apptest.New(t).Do("POST", "/api/github/token", map[string]string{"token": "ghp_personal"}).
		Expect(http.StatusUnauthorized, "unauthorized")
}
