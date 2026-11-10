package github_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/apptest"
)

func TestOAuth(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	expect := func(want bool) {
		t.Helper()
		// Public: sign-in asks it before there is a session.
		for _, cookies := range [][]*http.Cookie{{session}, nil, {{Name: "session", Value: "expired"}}} {
			res := env.Do("GET", "/api/github/oauth", nil, cookies...)
			res.Expect(http.StatusOK, "")
			var out map[string]any
			res.JSON(&out)
			if len(out) != 1 || out["enabled"] != want {
				t.Fatalf("GET /api/github/oauth with %v = %s, want {\"enabled\": %v}", cookies, res.Body, want)
			}
		}
	}
	expect(true)
	for _, unset := range []func(*app.Config){
		func(c *app.Config) { c.GitHubClientID = "" },
		func(c *app.Config) { c.GitHubClientSecret = "" },
	} {
		cfg := env.Config
		unset(&cfg)
		env.Server = httptest.NewServer(app.New(env.DB, cfg))
		t.Cleanup(env.Server.Close)
		expect(false)
	}
}
