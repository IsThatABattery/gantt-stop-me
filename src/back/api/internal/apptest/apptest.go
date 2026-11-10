// Package apptest runs the whole API on a fresh Postgres schema, for endpoint tests.
package apptest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/automation"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/planning"
	"gantt-stop-me/api/internal/runner/runnertest"
	"gantt-stop-me/api/internal/secret"
	"gantt-stop-me/api/internal/store"
	"gantt-stop-me/api/migrations"
)

// PublicURL is the app's URL in tests. Do sends it as the Origin header.
const PublicURL = "http://app.test"

// Env is a running API, its database, and the fake GitHub and runner it talks to.
type Env struct {
	t      testing.TB
	DB     *pgxpool.Pool
	Server *httptest.Server
	Config app.Config
	GitHub *githubtest.Fake
	Runner *runnertest.Fake // at Config.RunnerURL, so automation is on unless a test clears that
	// Automation is the API's automation. Its poller and queue worker never tick in tests: Automation.Poll runs one
	// poll of a project, Automation.Work one pass of the worker, and Automation.Wait waits for the webhook deliveries
	// already answered.
	Automation *automation.Automation
	// Planning is the API's planning. Its planner worker never ticks in tests: Planning.Work runs one pass.
	Planning *planning.Planning
	// Credentials reads users' GitHub tokens as the API does, refreshing expiring OAuth tokens.
	Credentials *github.Credentials
}

// New starts the API on a new, migrated schema, with OAuth linking enabled against a fake GitHub.
// configure, if given, changes the config before the API starts. All are torn down when the test ends.
func New(t testing.TB, configure ...func(*app.Config)) *Env {
	t.Helper()
	key := make([]byte, 32)
	rand.Read(key)
	box, err := secret.New(key)
	if err != nil {
		t.Fatal(err)
	}
	db := DB(t)
	gh := githubtest.New(t)
	run := runnertest.New(t)
	cfg := app.Config{
		PublicURL:                PublicURL,
		Secrets:                  box,
		GitHub:                   &github.Client{API: gh.Server.URL, Web: gh.Server.URL, HTTP: gh.Server.Client()},
		GitHubClientID:           gh.ClientID,
		GitHubClientSecret:       gh.ClientSecret,
		AllowPrivateProviderURLs: true, // fake provider endpoints listen on 127.0.0.1
		RunnerURL:                run.Server.URL,
		Runner:                   run.Client(),
		MaxConcurrentWorkflows:   1,
	}
	for _, c := range configure {
		c(&cfg)
	}
	api := app.New(db, cfg)
	srv := httptest.NewServer(api)
	// Cleanups run last first: the server stops taking deliveries, then the ones it answered finish, then the database goes.
	t.Cleanup(api.Automation.Wait)
	t.Cleanup(srv.Close)
	return &Env{t: t, DB: db, Server: srv, Config: cfg, GitHub: gh, Runner: run, Automation: api.Automation,
		Planning: api.Planning, Credentials: api.Credentials}
}

// DB returns a pool on a new schema with every migration applied, dropped when the test ends.
func DB(t testing.TB) *pgxpool.Pool {
	t.Helper()
	db := EmptyDB(t)
	if err := store.Migrate(context.Background(), db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return db
}

// EmptyDB returns a pool on a new schema with no migration applied, dropped when the test ends.
func EmptyDB(t testing.TB) *pgxpool.Pool {
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

// AgeSession makes the session as old as d: as though it started, and confirmed its user's password if it did, d ago.
func (e *Env) AgeSession(session *http.Cookie, d time.Duration) {
	e.t.Helper()
	sum := sha256.Sum256([]byte(session.Value))
	tag, err := e.DB.Exec(context.Background(), `
		update sessions set created_at = now() - make_interval(secs => $2),
			confirmed_at = case when confirmed_at is not null then now() - make_interval(secs => $2) end
		where id = $1`, hex.EncodeToString(sum[:]), d.Seconds())
	if err != nil || tag.RowsAffected() != 1 {
		e.t.Fatalf("age the session: %v, %d rows", err, tag.RowsAffected())
	}
}

// LinkGitHub adds a fake GitHub account with this login and links it to the session's user
// with a personal access token, which it returns.
func (e *Env) LinkGitHub(session *http.Cookie, login string) string {
	e.t.Helper()
	token := "ghp_" + rand.Text()
	e.GitHub.AddUser(token, GitHubID(login), login)
	e.Do("POST", "/api/github/token", map[string]string{"token": token}, session).Expect(http.StatusOK, "")
	return token
}

// LinkGitHubOAuth adds a fake GitHub account with this login and links it to the session's user with OAuth, as
// Settings does, and returns the access token GitHub issued.
func (e *Env) LinkGitHubOAuth(session *http.Cookie, login string) string {
	e.t.Helper()
	token, code := "gho_"+rand.Text(), rand.Text()
	e.GitHub.AddUser(token, GitHubID(login), login)
	e.GitHub.AddCode(code, token)
	state := e.Do("GET", "/api/github/link", nil, session).Cookie("github_state")
	if state == nil {
		e.t.Fatal("link set no state cookie")
	}
	res := e.Do("GET", "/api/github/callback?code="+code+"&state="+url.QueryEscape(state.Value), nil, session, state)
	if loc := res.Header.Get("Location"); res.Status != http.StatusFound || loc != PublicURL+"/settings?github=linked#github" {
		e.t.Fatalf("callback: got %d to %q, want a redirect to Settings with github=linked", res.Status, loc)
	}
	return token
}

// GitHubID is the fake GitHub account id LinkGitHub, LinkGitHubOAuth and SignInWithGitHub give login, so one login
// is one GitHub account across users.
func GitHubID(login string) int64 { return int64(crc32.ChecksumIEEE([]byte(login))) }

// SignInWithGitHub signs in with the fake GitHub account login, whose verified primary email it makes email, as the
// login page's Continue with GitHub does, and returns the session cookie and the access token GitHub issued. The
// first sign-in of an account no user is linked to makes one with that email and no password.
func (e *Env) SignInWithGitHub(login, email string) (*http.Cookie, string) {
	e.t.Helper()
	token, code := "gho_"+rand.Text(), rand.Text()
	e.GitHub.AddUser(token, GitHubID(login), login)
	e.GitHub.SetEmails(GitHubID(login), githubtest.Email{Email: email, Primary: true, Verified: true})
	e.GitHub.AddCode(code, token)
	state := e.Do("GET", "/api/auth/github", nil).Cookie("github_state")
	if state == nil {
		e.t.Fatal("sign-in set no state cookie")
	}
	res := e.Do("GET", "/api/github/callback?code="+code+"&state="+url.QueryEscape(state.Value), nil, state)
	if loc := res.Header.Get("Location"); res.Status != http.StatusFound || loc != PublicURL+"/" {
		e.t.Fatalf("callback: got %d to %q, want a redirect home", res.Status, loc)
	}
	session := res.Cookie(accounts.SessionCookie)
	if session == nil {
		e.t.Fatal("sign-in set no session cookie")
	}
	return session, token
}

// RefreshDuring has another request refresh the session user's OAuth token the first time GitHub is asked route
// pattern, as the fake registers it, before GitHub serves it, so GitHub refuses the token that request was made with:
// GitHub retires a token as it refreshes it. The user must be linked with OAuth while GitHub.ExpireTokens has set an
// expiry. It returns a function that returns the new token, failing the test unless the refresh worked.
func (e *Env) RefreshDuring(pattern string, session *http.Cookie) (refreshed func() string) {
	e.t.Helper()
	var me struct{ ID int64 }
	e.Do("GET", "/api/me", nil, session).JSON(&me)
	var once sync.Once
	var token string
	var err error
	e.GitHub.Before(pattern, func() {
		once.Do(func() {
			ctx := context.Background()
			_, err = e.DB.Exec(ctx, `update github_links set expires_at = now() + interval '1 minute' where user_id = $1`, me.ID)
			if err == nil {
				token, err = e.Credentials.Token(ctx, me.ID)
			}
		})
	})
	return func() string {
		e.t.Helper()
		if err != nil || token == "" {
			e.t.Fatalf("the other request's refresh: Token = %q, %v; want a new token", token, err)
		}
		return token
	}
}

// AddProject adds the fake repository owner/name, which the session's GitHub account must be able to write to
// or to see as an existing project, through POST /api/projects, and returns the project's id.
func (e *Env) AddProject(session *http.Cookie, owner, name string) int64 {
	e.t.Helper()
	res := e.Do("POST", "/api/projects", map[string]string{"url": "https://github.com/" + owner + "/" + name}, session)
	if res.Status != http.StatusCreated && res.Status != http.StatusOK {
		e.t.Fatalf("add project %s/%s: got %d %s", owner, name, res.Status, res.Body)
	}
	var p struct{ ID int64 }
	res.JSON(&p)
	return p.ID
}
