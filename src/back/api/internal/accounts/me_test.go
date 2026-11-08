package accounts_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
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
	sum := sha256.Sum256([]byte(session.Value))
	var raw, hashed int
	err := env.DB.QueryRow(context.Background(),
		`select count(*) filter (where id = $1), count(*) filter (where id = $2) from sessions`,
		session.Value, hex.EncodeToString(sum[:])).Scan(&raw, &hashed)
	if err != nil {
		t.Fatal(err)
	}
	if raw != 0 || hashed != 1 {
		t.Fatalf("rows with raw cookie = %d, with its sha256 = %d; want 0 and 1", raw, hashed)
	}
}

func TestEditMeName(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	name := func(res *apptest.Response) string {
		t.Helper()
		res.Expect(http.StatusOK, "")
		var me struct {
			Email string
			Name  json.RawMessage
		}
		res.JSON(&me)
		if me.Email != "ada@example.com" {
			t.Fatalf("answer = %+v, want the user as GET /api/me has them", me)
		}
		return string(me.Name)
	}

	if got := name(env.Do("GET", "/api/me", nil, session)); got != "null" {
		t.Fatalf("name before setting = %s, want null", got)
	}
	if got := name(env.Do("PATCH", "/api/me", map[string]any{"name": "  Ada Lovelace \n"}, session)); got != `"Ada Lovelace"` {
		t.Fatalf("PATCH answered name %s, want it trimmed", got)
	}
	if got := name(env.Do("GET", "/api/me", nil, session)); got != `"Ada Lovelace"` {
		t.Fatalf("name after setting = %s", got)
	}
	long := strings.Repeat("é", 100) // characters, not bytes
	if got := name(env.Do("PATCH", "/api/me", map[string]any{"name": long}, session)); got != `"`+long+`"` {
		t.Fatalf("100 characters: name = %s", got)
	}
	if got := name(env.Do("PATCH", "/api/me", map[string]any{"name": nil}, session)); got != "null" {
		t.Fatalf("name after clearing = %s, want null", got)
	}
}

func TestEditMeRefusesBadNames(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	for _, bad := range []string{"", "   ", strings.Repeat("a", 101), "Ada\tLovelace", "Ada\x00",
		"\u200b", "\u202eadmin", "a\u2028b", "a\u2029b", "\ufeff", " \u3000 "} {
		env.Do("PATCH", "/api/me", map[string]any{"name": bad}, session).Expect(http.StatusBadRequest, "invalid_name")
	}
	env.Do("PATCH", "/api/me", map[string]any{"name": 7}, session).Expect(http.StatusBadRequest, "bad_request")
	var name *string
	env.DB.QueryRow(context.Background(), `select name from users`).Scan(&name)
	if name != nil {
		t.Fatalf("name = %q after refused edits, want none", *name)
	}
	env.Do("PATCH", "/api/me", map[string]any{"name": "Ada"}).Expect(http.StatusUnauthorized, "unauthorized")
}
