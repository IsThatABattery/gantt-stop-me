package accounts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestMe(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	res := env.Do("GET", "/api/me", nil, session)
	res.Expect(http.StatusOK, "")
	var u struct {
		ID    int64
		Email string
	}
	res.JSON(&u)
	if u.ID == 0 || u.Email != "ada@example.com" {
		t.Fatalf("me = %+v", u)
	}
}

func TestMeShowsLinkedGitHub(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	github := func() string {
		res := env.Do("GET", "/api/me", nil, session)
		res.Expect(http.StatusOK, "")
		var me struct{ GitHub json.RawMessage }
		res.JSON(&me)
		return string(me.GitHub)
	}

	if got := github(); got != "null" {
		t.Fatalf("github before linking = %s, want null", got)
	}
	env.LinkGitHub(session, "octocat")
	if got := github(); got != `{"login":"octocat","kind":"pat"}` {
		t.Fatalf("github after linking = %s", got)
	}
	env.Do("DELETE", "/api/github", nil, session).Expect(http.StatusNoContent, "")
	if got := github(); got != "null" {
		t.Fatalf("github after unlinking = %s, want null", got)
	}
}

func TestMeNeedsValidSession(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	env.Do("GET", "/api/me", nil).Expect(http.StatusUnauthorized, "unauthorized")
	env.Do("GET", "/api/me", nil, &http.Cookie{Name: "session", Value: "forged"}).Expect(http.StatusUnauthorized, "unauthorized")

	env.DB.Exec(context.Background(), `update sessions set expires_at = now() - interval '1 second'`)
	env.Do("GET", "/api/me", nil, session).Expect(http.StatusUnauthorized, "unauthorized")
}

func TestSessionTokenIsStoredHashed(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	var n int
	env.DB.QueryRow(context.Background(), `select count(*) from sessions where id = $1`, session.Value).Scan(&n)
	if n != 0 {
		t.Fatal("sessions table holds the raw cookie value")
	}
}
