package github_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/apptest"
)

func TestLink(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	res := env.Do("GET", "/api/github/link", nil, session)
	res.Expect(http.StatusFound, "")
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if got := loc.Scheme + "://" + loc.Host + loc.Path; got != env.GitHub.Server.URL+"/login/oauth/authorize" {
		t.Fatalf("redirect to %s, want GitHub's authorize page", got)
	}
	q := loc.Query()
	want := map[string]string{
		"client_id":    env.Config.GitHubClientID,
		"redirect_uri": apptest.PublicURL + "/api/github/callback",
		"scope":        "repo workflow",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}

	c := res.Cookie("github_state")
	if c == nil || c.Value == "" || c.Value != q.Get("state") {
		t.Fatalf("state cookie %+v does not match state %q", c, q.Get("state"))
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure || c.Path != "/api/github/callback" || c.MaxAge != 600 {
		t.Fatalf("state cookie = %+v, want HttpOnly, SameSite=Lax, not Secure on http, Path=/api/github/callback, MaxAge=600", c)
	}
	if again := env.Do("GET", "/api/github/link", nil, session).Cookie("github_state"); again.Value == c.Value {
		t.Fatal("two links got the same state")
	}
}

func TestLinkSignedOut(t *testing.T) {
	env := apptest.New(t)
	for name, cookies := range map[string][]*http.Cookie{
		"no session":      nil,
		"expired session": {{Name: "session", Value: "expired"}},
	} {
		// Outside t.Run, because env.Do fails the parent test.
		res := env.Do("GET", "/api/github/link", nil, cookies...)
		t.Run(name, func(t *testing.T) {
			res.Expect(http.StatusFound, "")
			if loc, want := res.Header.Get("Location"), apptest.PublicURL+"/settings?github=error#github"; loc != want {
				t.Fatalf("redirect to %s, want %s", loc, want)
			}
			if c := res.Cookie("github_state"); c != nil && c.MaxAge >= 0 {
				t.Fatalf("signed-out link set the state cookie %+v", c)
			}
		})
	}
}

func TestLinkWhenOAuthIsNotSetUp(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	for _, unset := range []func(*app.Config){
		func(c *app.Config) { c.GitHubClientID = "" },
		func(c *app.Config) { c.GitHubClientSecret = "" },
	} {
		cfg := env.Config
		unset(&cfg)
		env.Server = httptest.NewServer(app.New(env.DB, cfg))
		t.Cleanup(env.Server.Close)

		res := env.Do("GET", "/api/github/link", nil, session)
		res.Expect(http.StatusFound, "")
		if loc, want := res.Header.Get("Location"), apptest.PublicURL+"/settings?github=unavailable#github"; loc != want {
			t.Fatalf("redirect to %s, want %s", loc, want)
		}
		if res.Cookie("github_state") != nil {
			t.Fatal("unconfigured link set a state cookie")
		}
	}
}
