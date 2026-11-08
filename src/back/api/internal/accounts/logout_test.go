package accounts_test

import (
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestLogout(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	res := env.Do("POST", "/api/auth/logout", nil, session)
	res.Expect(http.StatusNoContent, "")
	if c := res.Cookie("session"); c == nil || c.MaxAge >= 0 {
		t.Fatalf("logout cookie = %+v, want it deleted", c)
	}
	// The old cookie must be dead on the server, not only in the browser.
	env.Do("GET", "/api/me", nil, session).Expect(http.StatusUnauthorized, "unauthorized")
}

func TestLogoutWithoutSession(t *testing.T) {
	apptest.New(t).Do("POST", "/api/auth/logout", nil).Expect(http.StatusNoContent, "")
}

func TestLogoutEndsOnlyThatSession(t *testing.T) {
	env := apptest.New(t)
	first := env.SignUp("ada@example.com", "correct horse")
	second := env.Do("POST", "/api/auth/login", map[string]string{"email": "ada@example.com", "password": "correct horse"}).Cookie("session")

	env.Do("POST", "/api/auth/logout", nil, first).Expect(http.StatusNoContent, "")
	env.Do("GET", "/api/me", nil, second).Expect(http.StatusOK, "")
}
