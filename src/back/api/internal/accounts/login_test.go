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

// An account made by signing in with GitHub has no password until its user sets one, and no password logs in to it.
func TestLoginRefusesAnAccountWithNoPassword(t *testing.T) {
	env := apptest.New(t)
	env.SignUp("bob@example.com", "correct horse")
	env.SignInWithGitHub("octocat", "ada@example.com")

	wrong := env.Do("POST", "/api/auth/login", map[string]string{"email": "bob@example.com", "password": "wrong horse"})
	wrong.Expect(http.StatusUnauthorized, "invalid_credentials")
	for _, password := range []string{"", "correct horse"} {
		res := env.Do("POST", "/api/auth/login", map[string]string{"email": "ada@example.com", "password": password})
		res.Expect(http.StatusUnauthorized, "invalid_credentials")
		if string(res.Body) != string(wrong.Body) {
			t.Fatalf("no password %s and wrong password %s answer differently", res.Body, wrong.Body)
		}
		if res.Cookie("session") != nil {
			t.Fatal("failed login set a session cookie")
		}
	}
}
