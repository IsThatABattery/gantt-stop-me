// Package apptest runs the whole API on a fresh Postgres schema, for endpoint tests.
package apptest

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/secret"
	"gantt-stop-me/api/internal/store"
	"gantt-stop-me/api/migrations"
)

// PublicURL is the app's URL in tests. Do sends it as the Origin header.
const PublicURL = "http://app.test"

// Env is a running API and its database.
type Env struct {
	t      testing.TB
	DB     *pgxpool.Pool
	Server *httptest.Server
	Config app.Config
}

// New starts the API on a new, migrated schema. Both are torn down when the test ends.
func New(t testing.TB) *Env {
	t.Helper()
	key := make([]byte, 32)
	rand.Read(key)
	box, err := secret.New(key)
	if err != nil {
		t.Fatal(err)
	}
	db := DB(t)
	cfg := app.Config{PublicURL: PublicURL, Secrets: box}
	srv := httptest.NewServer(app.New(db, cfg))
	t.Cleanup(srv.Close)
	return &Env{t: t, DB: db, Server: srv, Config: cfg}
}

// DB returns a pool on a new schema with every migration applied, dropped when the test ends.
func DB(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL is not set. Run tests with: docker compose run --rm api-test")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ToLower(rand.Text())
	if _, err := admin.Exec(ctx, "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec(ctx, "drop schema "+schema+" cascade")
		admin.Close(ctx)
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := store.Migrate(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return db
}

// Response is a finished request with its body read.
type Response struct {
	t       testing.TB
	Status  int
	Header  http.Header
	Body    []byte
	Cookies []*http.Cookie
}

var client = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// Do sends a request the way the browser app would: from PublicURL, with a JSON body and cookies.
// body may be nil, a string sent as-is, or a value encoded as JSON. Redirects are not followed.
func (e *Env) Do(method, path string, body any, cookies ...*http.Cookie) *Response {
	e.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	default:
		data, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, e.Server.URL+path, r)
	if err != nil {
		e.t.Fatal(err)
	}
	req.Header.Set("Origin", PublicURL)
	if r != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}
	res, err := client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return &Response{t: e.t, Status: res.StatusCode, Header: res.Header, Body: data, Cookies: res.Cookies()}
}

// JSON decodes the body into v, failing the test if it is not JSON.
func (r *Response) JSON(v any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		r.t.Fatalf("response %d %q is not JSON: %v", r.Status, r.Body, err)
	}
}

// ErrorCode returns error.code from an error response, or "" if the body has none.
func (r *Response) ErrorCode() string {
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	json.Unmarshal(r.Body, &body)
	return body.Error.Code
}

// Cookie returns the cookie the response set with this name, or nil.
func (r *Response) Cookie(name string) *http.Cookie {
	for _, c := range r.Cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Expect fails the test unless the response has this status and, when code is not "", this error code.
func (r *Response) Expect(status int, code string) {
	r.t.Helper()
	if r.Status != status || (code != "" && r.ErrorCode() != code) {
		r.t.Fatalf("got %d %s, want %d %s", r.Status, r.Body, status, code)
	}
}

// SignUp creates an account and returns its session cookie.
func (e *Env) SignUp(email, password string) *http.Cookie {
	e.t.Helper()
	res := e.Do("POST", "/api/auth/signup", map[string]string{"email": email, "password": password})
	res.Expect(http.StatusCreated, "")
	c := res.Cookie(accounts.SessionCookie)
	if c == nil {
		e.t.Fatal("signup set no session cookie")
	}
	return c
}
