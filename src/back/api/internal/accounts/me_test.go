package accounts_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
