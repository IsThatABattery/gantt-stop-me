package accounts_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"gantt-stop-me/api/internal/apptest"
)

// confirmed reports whether session has confirmed its user's password in the last minute.
func confirmed(t *testing.T, env *apptest.Env, session *http.Cookie) bool {
	t.Helper()
	sum := sha256.Sum256([]byte(session.Value))
	var ok bool
	if err := env.DB.QueryRow(context.Background(), `select coalesce(confirmed_at > now() - interval '1 minute', false) from sessions where id = $1`,
		hex.EncodeToString(sum[:])).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

// expectMessage fails the test unless res's error message is want.
func expectMessage(t *testing.T, res *apptest.Response, want string) {
	t.Helper()
	var body struct{ Error struct{ Message string } }
	res.JSON(&body)
	if body.Error.Message != want {
		t.Fatalf("message = %q, want %q", body.Error.Message, want)
	}
}

// Confirming marks only the session it came from, so a stolen one stays stale while its user confirms elsewhere.
func TestConfirm(t *testing.T) {
	env := apptest.New(t)
	current := env.SignUp("ada@example.com", "correct horse")
	other := login(env, "correct horse").Cookie("session")
	env.AgeSession(current, 24*time.Hour)
	env.AgeSession(other, 24*time.Hour)

	env.Do("POST", "/api/auth/confirm", map[string]string{"password": "correct horse"}, current).Expect(http.StatusNoContent, "")
	if !confirmed(t, env, current) {
		t.Fatal("the session that confirmed is not marked")
	}
	if confirmed(t, env, other) {
		t.Fatal("another session of the user is marked too")
	}
	env.Do("GET", "/api/me", nil, other).Expect(http.StatusOK, "")
}

func TestConfirmWrongPassword(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	for _, password := range []string{"wrong horse", ""} {
		res := env.Do("POST", "/api/auth/confirm", map[string]string{"password": password}, session)
		res.Expect(http.StatusForbidden, "wrong_password")
		expectMessage(t, res, "Your password is wrong.")
	}
	if confirmed(t, env, session) {
		t.Fatal("a wrong password marked the session")
	}
}

// An account with no password signs in only with GitHub, so a session from signing in with GitHub again is the only
// recent one it gets: confirming changes nothing, and a stale session still cannot set a first password.
func TestConfirmWithNoPassword(t *testing.T) {
	env := apptest.New(t)
	stale, _ := env.SignInWithGitHub("octocat", "octo@example.com")
	env.AgeSession(stale, freshFor+time.Minute)

	for _, password := range []string{"", "correct horse"} {
		res := env.Do("POST", "/api/auth/confirm", map[string]string{"password": password}, stale)
		res.Expect(http.StatusConflict, "password_required")
		expectMessage(t, res, "This account has no password. Sign in with GitHub again instead.")
	}
	if confirmed(t, env, stale) {
		t.Fatal("confirming with no password marked the session")
	}
	env.Do("PATCH", "/api/me/password", map[string]string{"newPassword": "battery staple"}, stale).
		Expect(http.StatusForbidden, "reauth_required")
}

func TestConfirmSignedOut(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	env.Do("POST", "/api/auth/confirm", map[string]string{"password": "correct horse"}).Expect(http.StatusUnauthorized, "unauthorized")
	env.Do("POST", "/api/auth/confirm", map[string]string{"password": "correct horse"}, &http.Cookie{Name: "session", Value: "expired"}).
		Expect(http.StatusUnauthorized, "unauthorized")
	env.Do("POST", "/api/auth/logout", nil, session).Expect(http.StatusNoContent, "")
	env.Do("POST", "/api/auth/confirm", map[string]string{"password": "correct horse"}, session).Expect(http.StatusUnauthorized, "unauthorized")
}

func TestConfirmRejectsBadJSON(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.Do("POST", "/api/auth/confirm", "{", session).Expect(http.StatusBadRequest, "bad_request")
}

// Another site could otherwise confirm with a password it guessed, from the user's own browser and session.
func TestConfirmChecksOrigin(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.AgeSession(session, 24*time.Hour)

	for _, origin := range []string{"", "http://evil.test"} {
		req, err := http.NewRequest("POST", env.Server.URL+"/api/auth/confirm", strings.NewReader(`{"password": "correct horse"}`))
		if err != nil {
			t.Fatal(err)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(session)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var body struct{ Error struct{ Code string } }
		json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden || body.Error.Code != "bad_origin" {
			t.Fatalf("confirm with Origin %q = %d %s, want 403 bad_origin", origin, res.StatusCode, body.Error.Code)
		}
	}
	if confirmed(t, env, session) {
		t.Fatal("a cross-origin confirm marked the session")
	}
}
