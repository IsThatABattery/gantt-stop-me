package accounts_test

import (
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func login(env *apptest.Env, password string) *apptest.Response {
	return env.Do("POST", "/api/auth/login", map[string]string{"email": "ada@example.com", "password": password})
}

func TestChangePassword(t *testing.T) {
	env := apptest.New(t)
	current := env.SignUp("ada@example.com", "correct horse")
	other := login(env, "correct horse").Cookie("session")

	env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "correct horse", "newPassword": "battery staple"}, current).
		Expect(http.StatusNoContent, "")

	login(env, "correct horse").Expect(http.StatusUnauthorized, "invalid_credentials")
	login(env, "battery staple").Expect(http.StatusOK, "")
	env.Do("GET", "/api/me", nil, current).Expect(http.StatusOK, "")
	env.Do("GET", "/api/me", nil, other).Expect(http.StatusUnauthorized, "unauthorized")
}

func TestChangePasswordChecksCurrentPassword(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "wrong horse", "newPassword": "battery staple"}, session).
		Expect(http.StatusForbidden, "wrong_password")
	login(env, "correct horse").Expect(http.StatusOK, "")
}

func TestChangePasswordValidatesInput(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "correct horse", "newPassword": "short"}, session).
		Expect(http.StatusBadRequest, "weak_password")
	env.Do("PATCH", "/api/me/password", "{", session).Expect(http.StatusBadRequest, "bad_request")
	env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "correct horse", "newPassword": "battery staple"}).
		Expect(http.StatusUnauthorized, "unauthorized")
}
