package github_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/oauth"
)

func TestSignIn(t *testing.T) {
	env := apptest.New(t)
	for name, cookies := range map[string][]*http.Cookie{
		"signed out": nil,
		"signed in":  {env.SignUp("ada@example.com", "correct horse")},
	} {
		// Outside t.Run, because env.Do fails the parent test.
		res := env.Do("GET", "/api/auth/github", nil, cookies...)
		again := env.Do("GET", "/api/auth/github", nil, cookies...).Cookie("github_state")
		t.Run(name, func(t *testing.T) {
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
				"client_id": env.Config.GitHubClientID,
				// The callback linking uses, so the OAuth App needs no other.
				"redirect_uri": apptest.PublicURL + "/api/github/callback",
				"scope":        "repo workflow user:email",
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
			// Without next, an earlier sign-in's must not send this one anywhere.
			if next := res.Cookie("github_next"); next == nil || next.MaxAge >= 0 || next.Path != "/api/github/callback" {
				t.Fatalf("next cookie = %+v, want it deleted", next)
			}
			if kind := oauth.KindOf(c.Value); kind != oauth.SignIn {
				t.Fatalf("state %q is for a %q flow, want a sign-in", c.Value, kind)
			}
			if again == nil || again.Value == c.Value {
				t.Fatal("two sign-ins got the same state")
			}
		})
	}
}

func TestSignInWhenOAuthIsNotSetUp(t *testing.T) {
	env := apptest.New(t)
	for _, unset := range []func(*app.Config){
		func(c *app.Config) { c.GitHubClientID = "" },
		func(c *app.Config) { c.GitHubClientSecret = "" },
	} {
		cfg := env.Config
		unset(&cfg)
		env.Server = httptest.NewServer(app.New(env.DB, cfg))
		t.Cleanup(env.Server.Close)

		res := env.Do("GET", "/api/auth/github", nil)
		res.Expect(http.StatusFound, "")
		if loc, want := res.Header.Get("Location"), apptest.PublicURL+"/login?github=unavailable"; loc != want {
			t.Fatalf("redirect to %s, want %s", loc, want)
		}
		if res.Cookie("github_state") != nil {
			t.Fatal("unconfigured sign-in set a state cookie")
		}
	}
}
