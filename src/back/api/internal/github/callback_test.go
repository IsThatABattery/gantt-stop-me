package github_test

import (
	"net/http"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

// startLink begins linking with OAuth and returns the state cookie the browser brings back to the callback.
func startLink(t *testing.T, env *apptest.Env, session *http.Cookie) *http.Cookie {
	t.Helper()
	c := env.Do("GET", "/api/github/link", nil, session).Cookie("github_state")
	if c == nil {
		t.Fatal("link set no state cookie")
	}
	return c
}

// expectSettings fails the test unless res sends the browser to Settings with this outcome and deletes the state cookie.
func expectSettings(t *testing.T, res *apptest.Response, outcome string) {
	t.Helper()
	res.Expect(http.StatusFound, "")
	if loc, want := res.Header.Get("Location"), apptest.PublicURL+"/settings?github="+outcome; loc != want {
		t.Fatalf("redirect to %s, want %s", loc, want)
	}
	if c := res.Cookie("github_state"); c == nil || c.MaxAge >= 0 {
		t.Fatalf("state cookie = %+v, want it deleted", c)
	}
}

func TestCallback(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.AddUser("gho_oauth_token", 42, "octocat")
	env.GitHub.AddCode("good-code", "gho_oauth_token")
	state := startLink(t, env, session)

	res := env.Do("GET", "/api/github/callback?code=good-code&state="+state.Value, nil, session, state)
	expectSettings(t, res, "linked")
	links := stored(t, env)
	if len(links) != 1 || links[0] != (storedLink{GitHubID: 42, Login: "octocat", Kind: "oauth", Token: "gho_oauth_token"}) {
		t.Fatalf("github_links = %+v, want one oauth link to octocat", links)
	}
}

func TestCallbackFailuresStoreNothing(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.AddUser("gho_oauth_token", 42, "octocat")
	cases := []struct {
		name   string
		query  string // STATE stands for the state link issued
		cookie string // the github_state the browser sends: "link" for the one link set, "none", or "empty"
	}{
		{"wrong state", "code=good-code&state=forged", "link"},
		{"missing state", "code=good-code", "link"},
		{"missing cookie", "code=good-code&state=STATE", "none"},
		{"empty state and cookie", "code=good-code&state=", "empty"},
		{"user denied", "error=access_denied&state=STATE", "link"},
		{"bad code", "code=bad-code&state=STATE", "link"},
		{"token GitHub rejects", "code=orphan-code&state=STATE", "link"},
	}
	for _, c := range cases {
		env.GitHub.AddCode("good-code", "gho_oauth_token")
		env.GitHub.AddCode("orphan-code", "gho_unknown")
		state := startLink(t, env, session)
		cookies := []*http.Cookie{session}
		switch c.cookie {
		case "link":
			cookies = append(cookies, state)
		case "empty":
			cookies = append(cookies, &http.Cookie{Name: "github_state", Value: ""})
		}

		// Outside t.Run, because env.Do fails the parent test.
		res := env.Do("GET", "/api/github/callback?"+strings.ReplaceAll(c.query, "STATE", state.Value), nil, cookies...)
		t.Run(c.name, func(t *testing.T) {
			expectSettings(t, res, "error")
			if links := stored(t, env); len(links) != 0 {
				t.Fatalf("github_links = %+v, want none", links)
			}
		})
	}
}

func TestCallbackNeedsSession(t *testing.T) {
	apptest.New(t).Do("GET", "/api/github/callback?code=good-code&state=x", nil).Expect(http.StatusUnauthorized, "unauthorized")
}
