package accounts_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/apptest"
)

func TestSignup(t *testing.T) {
	env := apptest.New(t)
	res := env.Do("POST", "/api/auth/signup", map[string]string{"email": "ada@example.com", "password": "correct horse"})
	res.Expect(http.StatusCreated, "")
	var u struct {
		ID    int64
		Email string
	}
	res.JSON(&u)
	if u.ID == 0 || u.Email != "ada@example.com" {
		t.Fatalf("signup returned %+v", u)
	}

	c := res.Cookie("session")
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge <= 0 {
		t.Fatalf("session cookie = %+v, want HttpOnly, SameSite=Lax, Path=/, persistent", c)
	}
	env.Do("GET", "/api/me", nil, c).Expect(http.StatusOK, "")

	var hash string
	env.DB.QueryRow(context.Background(), `select password_hash from users where id = $1`, u.ID).Scan(&hash)
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("stored password %q is not an argon2id hash", hash)
	}
}

// apptest serves plain http, so this drives Accounts directly to reach the https case.
func TestSessionCookieSecureOnlyOnHTTPS(t *testing.T) {
	for _, tc := range []struct {
		url    string
		secure bool
	}{{"https://app.test", true}, {"http://app.test", false}} {
		mux := http.NewServeMux()
		accounts.New(apptest.DB(t), tc.url).Register(mux)
		req := httptest.NewRequest("POST", "/api/auth/signup", strings.NewReader(`{"email":"ada@example.com","password":"correct horse"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var c *http.Cookie
		for _, k := range rec.Result().Cookies() {
			if k.Name == accounts.SessionCookie {
				c = k
			}
		}
		if rec.Code != http.StatusCreated || c == nil || c.Secure != tc.secure {
			t.Fatalf("PUBLIC_URL %s: status %d, cookie %+v, want Secure=%v", tc.url, rec.Code, c, tc.secure)
		}
	}
}

func TestSignupRejectsTakenEmailIgnoringCase(t *testing.T) {
	env := apptest.New(t)
	env.SignUp("ada@example.com", "correct horse")
	env.Do("POST", "/api/auth/signup", map[string]string{"email": "ADA@example.com", "password": "other password"}).
		Expect(http.StatusConflict, "email_taken")
}

func TestSignupValidatesInput(t *testing.T) {
	env := apptest.New(t)
	cases := []struct {
		body any
		code string
	}{
		{map[string]string{"email": "not-an-email", "password": "correct horse"}, "invalid_email"},
		{map[string]string{"email": "Ada <ada@example.com>", "password": "correct horse"}, "invalid_email"},
		{map[string]string{"email": "", "password": "correct horse"}, "invalid_email"},
		{map[string]string{"email": "ada@example.com", "password": "short"}, "weak_password"},
		{"{not json", "bad_request"},
	}
	for _, c := range cases {
		env.Do("POST", "/api/auth/signup", c.body).Expect(http.StatusBadRequest, c.code)
	}
}

func TestSignupNeedsSameOrigin(t *testing.T) {
	env := apptest.New(t)
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/auth/signup",
		strings.NewReader(`{"email":"ada@example.com","password":"correct horse"}`))
	req.Header.Set("Origin", "http://evil.test")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin signup: status %d, want 403", res.StatusCode)
	}
}
