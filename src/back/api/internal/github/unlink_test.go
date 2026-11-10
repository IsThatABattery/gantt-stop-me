package github_test

import (
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestUnlink(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	env.LinkGitHub(ada, "ada-gh")
	env.LinkGitHub(bob, "bob-gh")
	addMembership(t, env, ada)
	addMembership(t, env, bob)

	env.Do("DELETE", "/api/github", nil, ada).Expect(http.StatusNoContent, "")
	if links := stored(t, env); len(links) != 1 || links[0].Login != "bob-gh" {
		t.Fatalf("github_links = %+v, want only bob's link", links)
	}
	if n := memberships(t, env, ada); n != 0 {
		t.Fatalf("unlinking kept %d project memberships", n)
	}
	if n := memberships(t, env, bob); n != 1 {
		t.Fatalf("bob has %d project memberships, want his 1 untouched", n)
	}
	env.Do("DELETE", "/api/github", nil, ada).Expect(http.StatusNoContent, "")
}

func TestUnlinkNeedsSession(t *testing.T) {
	apptest.New(t).Do("DELETE", "/api/github", nil).Expect(http.StatusUnauthorized, "unauthorized")
}

// A user made by signing in with GitHub has no password, so without the link they could not sign in.
func TestUnlinkNeedsAPassword(t *testing.T) {
	env := apptest.New(t)
	session, token := env.SignInWithGitHub("octocat", "octo@example.com")
	addMembership(t, env, session)

	res := env.Do("DELETE", "/api/github", nil, session)
	res.Expect(http.StatusConflict, "password_required")
	var body struct{ Error struct{ Message string } }
	res.JSON(&body)
	if want := "Set a password before unlinking GitHub, so you can still sign in."; body.Error.Message != want {
		t.Fatalf("message = %q, want %q", body.Error.Message, want)
	}
	if links := stored(t, env); len(links) != 1 || links[0].Token != token {
		t.Fatalf("github_links = %+v, want the link untouched", links)
	}
	if n := memberships(t, env, session); n != 1 {
		t.Fatalf("refused unlinking left %d project memberships, want 1", n)
	}

	env.Do("PATCH", "/api/me/password", map[string]string{"newPassword": "correct horse"}, session).Expect(http.StatusNoContent, "")
	env.Do("DELETE", "/api/github", nil, session).Expect(http.StatusNoContent, "")
	if links := stored(t, env); len(links) != 0 {
		t.Fatalf("github_links = %+v, want none", links)
	}
}
