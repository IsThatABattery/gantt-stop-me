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
