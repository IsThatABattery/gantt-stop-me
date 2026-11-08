package accounts_test

import (
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestLogin(t *testing.T) {
	env := apptest.New(t)
	env.SignUp("ada@example.com", "correct horse")

	res := env.Do("POST", "/api/auth/login", map[string]string{"email": "ADA@example.com", "password": "correct horse"})
	res.Expect(http.StatusOK, "")
	var u struct{ Email string }
	res.JSON(&u)
	if u.Email != "ada@example.com" {
		t.Fatalf("login returned email %q", u.Email)
	}
	env.Do("GET", "/api/me", nil, res.Cookie("session")).Expect(http.StatusOK, "")
}

func TestLoginRejectsBadCredentialsAlike(t *testing.T) {
	env := apptest.New(t)
	env.SignUp("ada@example.com", "correct horse")

	wrong := env.Do("POST", "/api/auth/login", map[string]string{"email": "ada@example.com", "password": "wrong horse"})
	unknown := env.Do("POST", "/api/auth/login", map[string]string{"email": "bob@example.com", "password": "correct horse"})
	wrong.Expect(http.StatusUnauthorized, "invalid_credentials")
	unknown.Expect(http.StatusUnauthorized, "invalid_credentials")
	if string(wrong.Body) != string(unknown.Body) {
		t.Fatalf("wrong password %s and unknown email %s answer differently", wrong.Body, unknown.Body)
	}
	if wrong.Cookie("session") != nil {
		t.Fatal("failed login set a session cookie")
	}
}

func TestLoginRejectsBadJSON(t *testing.T) {
	apptest.New(t).Do("POST", "/api/auth/login", "nope").Expect(http.StatusBadRequest, "bad_request")
}
