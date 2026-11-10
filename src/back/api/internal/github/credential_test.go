package github_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github"
)

type storedLink struct {
	GitHubID    int64
	Login, Kind string
	Token       string    // credential_enc, decrypted
	Refresh     string    // refresh_enc, decrypted, "" while null
	Expires     time.Time // expires_at, zero while null
}

// stored returns every row of github_links, failing the test if a credential is stored in plaintext.
func stored(t *testing.T, env *apptest.Env) []storedLink {
	t.Helper()
	rows, err := env.DB.Query(context.Background(), `select github_id, login, kind, credential_enc, refresh_enc, expires_at
		from github_links order by user_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var links []storedLink
	for rows.Next() {
		var l storedLink
		var sealed, sealedRefresh []byte
		var expires *time.Time
		if err := rows.Scan(&l.GitHubID, &l.Login, &l.Kind, &sealed, &sealedRefresh, &expires); err != nil {
			t.Fatal(err)
		}
		l.Token = open(t, env, l.Login, sealed)
		if sealedRefresh != nil {
			l.Refresh = open(t, env, l.Login, sealedRefresh)
		}
		if expires != nil {
			l.Expires = expires.UTC()
		}
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return links
}

// open decrypts a secret stored for login, failing the test if it is stored in plaintext.
func open(t *testing.T, env *apptest.Env, login string, sealed []byte) string {
	t.Helper()
	plain, err := env.Config.Secrets.Open(sealed)
	if err != nil {
		t.Fatalf("open secret of %s: %v", login, err)
	}
	if bytes.Contains(sealed, plain) {
		t.Fatalf("secret of %s is stored in plaintext", login)
	}
	return string(plain)
}

func userID(t *testing.T, env *apptest.Env, session *http.Cookie) int64 {
	t.Helper()
	var me struct{ ID int64 }
	env.Do("GET", "/api/me", nil, session).JSON(&me)
	return me.ID
}

// addMembership caches access for the session's user to a new project, as opening a board does.
func addMembership(t *testing.T, env *apptest.Env, session *http.Cookie) {
	t.Helper()
	_, err := env.DB.Exec(context.Background(), `
		with p as (
			insert into projects (github_repo_id, owner, name)
			values ((select coalesce(max(github_repo_id), 0) + 1 from projects), 'octo', 'hello') returning id)
		insert into project_members (project_id, user_id, permission, checked_at) select id, $1, 'write', now() from p`,
		userID(t, env, session))
	if err != nil {
		t.Fatal(err)
	}
}

// memberships counts the session user's cached project access.
func memberships(t *testing.T, env *apptest.Env, session *http.Cookie) int {
	t.Helper()
	var n int
	if err := env.DB.QueryRow(context.Background(), `select count(*) from project_members where user_id = $1`, userID(t, env, session)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestToken(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	token := env.LinkGitHub(ada, "ada-gh")
	ctx := context.Background()

	got, err := env.Credentials.Token(ctx, userID(t, env, ada))
	if err != nil || got != token {
		t.Fatalf("Token = %q, %v; want the linked token", got, err)
	}
	if _, err := env.Credentials.Token(ctx, userID(t, env, bob)); !errors.Is(err, github.ErrNotLinked) {
		t.Fatalf("Token for an unlinked user: err = %v, want ErrNotLinked", err)
	}
}

// expiringLink signs up name@example.com and links them with OAuth, as name-gh, to a token that expires in 8 hours,
// as GitHub's default is, and returns their session, their user id and the token.
func expiringLink(t *testing.T, env *apptest.Env, name string) (*http.Cookie, int64, string) {
	t.Helper()
	env.GitHub.ExpireTokens(8 * time.Hour)
	session := env.SignUp(name+"@example.com", "correct horse")
	return session, userID(t, env, session), env.LinkGitHubOAuth(session, name+"-gh")
}

// expiresIn moves the expiry of the user's stored token to d from now; a negative d has it expired.
func expiresIn(t *testing.T, env *apptest.Env, id int64, d time.Duration) {
	t.Helper()
	if _, err := env.DB.Exec(context.Background(), `update github_links set expires_at = $2 where user_id = $1`,
		id, time.Now().Add(d)); err != nil {
		t.Fatal(err)
	}
}

// credentials reads tokens through db as env's API does, as another api instance would: with gates and a backoff
// of its own.
func credentials(env *apptest.Env, db *pgxpool.Pool) *github.Credentials {
	return &github.Credentials{DB: db, Secrets: env.Config.Secrets, Client: env.Config.GitHub,
		ClientID: env.Config.GitHubClientID, ClientSecret: env.Config.GitHubClientSecret}
}

// tokenRequests counts the requests to GitHub's OAuth token endpoint so far, those Fail answered included.
func tokenRequests(env *apptest.Env) int {
	n := 0
	for _, r := range env.GitHub.Requests() {
		if r == "POST /login/oauth/access_token" {
			n++
		}
	}
	return n
}

// login is the GitHub login token authenticates as, or "" when GitHub refuses it.
func login(t *testing.T, env *apptest.Env, token string) string {
	t.Helper()
	u, err := env.Config.GitHub.User(context.Background(), token)
	if ghErr := (*github.Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusUnauthorized {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return u.Login
}

func TestTokenRefreshesNearExpiry(t *testing.T) {
	env := apptest.New(t)
	_, ada, token := expiringLink(t, env, "ada")
	ctx := context.Background()
	before := stored(t, env)[0]
	if before.Refresh == "" || time.Until(before.Expires) < 8*time.Hour-time.Minute || time.Until(before.Expires) > 8*time.Hour {
		t.Fatalf("stored link = %+v, want a refresh token and an expiry 8 hours away", before)
	}

	if got, err := env.Credentials.Token(ctx, ada); err != nil || got != token || env.GitHub.Refreshes() != 0 {
		t.Fatalf("Token 8 hours before expiry = %q, %v after %d refreshes; want the token, unrefreshed", got, err, env.GitHub.Refreshes())
	}
	for i, d := range []time.Duration{4 * time.Minute, -time.Minute} {
		expiresIn(t, env, ada, d)
		got, err := env.Credentials.Token(ctx, ada)
		if err != nil || got == token || env.GitHub.Refreshes() != i+1 {
			t.Fatalf("Token %v from expiry = %q, %v after %d refreshes; want a new token from one more refresh", d, got, err, env.GitHub.Refreshes())
		}
		after := stored(t, env)[0]
		if after.Token != got || after.Refresh == "" || after.Refresh == before.Refresh ||
			time.Until(after.Expires) < 8*time.Hour-time.Minute || after.Login != "ada-gh" || after.Kind != "oauth" {
			t.Fatalf("stored link = %+v, want the new token, a new refresh token and an expiry 8 hours away", after)
		}
		if login(t, env, token) != "" || login(t, env, got) != "ada-gh" {
			t.Fatal("GitHub still takes the old token, or not the new one")
		}
		if again, err := env.Credentials.Token(ctx, ada); err != nil || again != got || env.GitHub.Refreshes() != i+1 {
			t.Fatalf("Token after the refresh = %q, %v; want the new token, unrefreshed", again, err)
		}
		token, before = got, after
	}
}

func TestTokenNeverRefreshesTokensThatDoNotExpire(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	tokens := map[int64]string{userID(t, env, ada): env.LinkGitHubOAuth(ada, "ada-gh"), userID(t, env, bob): env.LinkGitHub(bob, "bob-gh")}
	for _, l := range stored(t, env) {
		if l.Refresh != "" || !l.Expires.IsZero() {
			t.Fatalf("stored %s link = %+v, want no refresh token or expiry", l.Kind, l)
		}
	}
	for id, token := range tokens {
		if got, err := env.Credentials.Token(context.Background(), id); err != nil || got != token {
			t.Fatalf("Token = %q, %v; want the linked token", got, err)
		}
	}
	if n := env.GitHub.Refreshes(); n != 0 {
		t.Fatalf("%d refreshes, want none", n)
	}
}

func TestTokenRefreshesOnceAtATime(t *testing.T) {
	env := apptest.New(t)
	_, ada, token := expiringLink(t, env, "ada")
	expiresIn(t, env, ada, time.Minute)
	ctx := context.Background()
	// A pool of two of its own: its stats count only the reads' connections, and reads that held one while they
	// waited would use it up.
	cfg := env.DB.Config()
	cfg.MaxConns = 2
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	creds := credentials(env, db)

	// GitHub holds the refresh until the other reads wait for it, or the test ends, before the pool closes, which
	// waits for the refresh's connection.
	arrived, held := make(chan struct{}, 10), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(held) }) }
	t.Cleanup(release)
	env.GitHub.Before("POST /login/oauth/access_token", func() {
		arrived <- struct{}{}
		<-held
	})
	type result struct {
		token string
		err   error
	}
	const reads = 5
	results := make(chan result, reads)
	read := func(ctx context.Context, results chan<- result) {
		got, err := creds.Token(ctx, ada)
		results <- result{got, err}
	}
	go read(ctx, results)
	select {
	case <-arrived:
	case <-time.After(10 * time.Second):
		t.Fatal("no refresh reached GitHub")
	}
	// Each other read reads the link once, then waits for the refresh without a connection.
	base := db.Stat().AcquireCount()
	waiting := func(n int64) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); db.Stat().AcquireCount()-base < n || db.Stat().AcquiredConns() != 1; {
			if time.Now().After(deadline) {
				t.Fatalf("%d connections taken and %d held; want %d reads to have read the link, and to wait with the refresh's connection alone held",
					db.Stat().AcquireCount()-base, db.Stat().AcquiredConns(), n)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	for range reads - 1 {
		go read(ctx, results)
	}
	waiting(reads - 1)
	time.Sleep(50 * time.Millisecond)
	if n := db.Stat().AcquiredConns(); n != 1 {
		t.Fatalf("%d connections held while the reads wait, want the refresh's alone", n)
	}
	// A read whose caller gives up while it waits takes the stored token, which has a minute left.
	cctx, cancel := context.WithCancel(ctx)
	gaveUp := make(chan result, 1)
	go read(cctx, gaveUp)
	waiting(reads)
	cancel()
	if r := <-gaveUp; r.err != nil || r.token != token {
		t.Fatalf("Token for a caller that gave up = %q, %v; want the stored token", r.token, r.err)
	}

	release()
	want := ""
	for range reads {
		r := <-results
		if want == "" {
			want = r.token
		}
		if r.err != nil || r.token != want || r.token == token {
			t.Fatalf("Token = %q, %v; want the one new token %q for every read", r.token, r.err, want)
		}
	}
	if n := env.GitHub.Refreshes(); n != 1 || len(arrived) != 0 {
		t.Fatalf("%d refreshes, want 1", n)
	}
	if l := stored(t, env)[0]; l.Token != want {
		t.Fatalf("stored token %q, want the new one", l.Token)
	}
}

func TestTokenWhileAnotherInstanceRefreshes(t *testing.T) {
	env := apptest.New(t)
	_, ada, adaToken := expiringLink(t, env, "ada")
	_, bob, _ := expiringLink(t, env, "bob")
	expiresIn(t, env, ada, time.Minute)
	expiresIn(t, env, bob, -time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Hold both links as another api instance's refreshes would, for longer than a read waits for one.
	tx, err := env.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select from github_links for update`); err != nil {
		t.Fatal(err)
	}
	type result struct {
		token string
		err   error
	}
	adaRead, bobRead := make(chan result, 1), make(chan result, 1)
	before, start := tokenRequests(env), time.Now()
	for id, results := range map[int64]chan result{ada: adaRead, bob: bobRead} {
		go func() {
			got, err := env.Credentials.Token(ctx, id)
			results <- result{got, err}
		}()
	}
	a, b := <-adaRead, <-bobRead
	if waited := time.Since(start); waited < 5*time.Second || waited > 20*time.Second {
		t.Fatalf("the reads took %v, want them to wait out the 5-second lock timeout", waited)
	}
	if a.err != nil || a.token != adaToken {
		t.Fatalf("Token valid for a minute = %q, %v; want the stored token", a.token, a.err)
	}
	if !errors.Is(b.err, github.ErrUnavailable) || b.token != "" {
		t.Fatalf("Token expired = %q, %v; want ErrUnavailable", b.token, b.err)
	}
	if n := tokenRequests(env) - before; n != 0 {
		t.Fatalf("%d refresh requests, want none", n)
	}
}

func TestTokenWhenGitHubRefusesTheRefreshToken(t *testing.T) {
	env := apptest.New(t)
	_, ada, token := expiringLink(t, env, "ada")
	ctx := context.Background()
	// GitHub forgot the refresh token: it expired, or a refresh whose answer got lost used it up.
	if _, err := env.DB.Exec(ctx, `update github_links set refresh_enc = $2 where user_id = $1`,
		ada, env.Config.Secrets.Seal([]byte("ghr_unknown"))); err != nil {
		t.Fatal(err)
	}
	expiresIn(t, env, ada, time.Minute)
	want := stored(t, env)[0]
	want.Refresh = ""
	logs := captureLog(t)

	if got, err := env.Credentials.Token(ctx, ada); err != nil || got != token || env.GitHub.Refreshes() != 1 {
		t.Fatalf("Token = %q, %v after %d refreshes; want the stored token after one", got, err, env.GitHub.Refreshes())
	}
	if after := stored(t, env); len(after) != 1 || after[0] != want {
		t.Fatalf("github_links = %+v, want the link without its refresh token, %+v", after, want)
	}
	if !strings.Contains(logs.String(), `"bad_refresh_token"`) {
		t.Fatalf("the log does not say why:\n%s", logs)
	}
	// Later reads, of any instance, take the stored token as it is, even once it expires, without asking GitHub, and
	// GitHub's 401 for it asks the user to link again.
	for _, d := range []time.Duration{time.Minute, -time.Minute} {
		expiresIn(t, env, ada, d)
		if got, err := credentials(env, env.DB).Token(ctx, ada); err != nil || got != token || env.GitHub.Refreshes() != 1 {
			t.Fatalf("%v from expiry: Token = %q, %v after %d refreshes; want the stored token, unrefreshed", d, got, err, env.GitHub.Refreshes())
		}
	}
}

func TestTokenWhenItCannotRefresh(t *testing.T) {
	env := apptest.New(t)
	_, ada, token := expiringLink(t, env, "ada")
	ctx := context.Background()
	wrongSecret := func() *github.Credentials {
		c := credentials(env, env.DB)
		c.ClientSecret = "wrong"
		return c
	}
	unconfigured := credentials(env, env.DB)
	unconfigured.ClientSecret = ""
	refused := wrongSecret()
	// A refusal other than bad_refresh_token is the OAuth App's, which linking again does not mend: like GitHub out of
	// reach, it leaves the stored token in use until it expires.
	for _, c := range []struct {
		name        string
		credentials *github.Credentials
		expires     time.Duration
		requests    int
		unavailable bool // Token answers ErrUnavailable, not the stored token
	}{
		{"GitHub refuses the OAuth App's credentials", refused, time.Minute, 1, false},
		{"again within 30 seconds", refused, time.Minute, 0, false},
		{"again within 30 seconds, for an expired token", refused, -time.Minute, 0, true},
		{"GitHub refuses the OAuth App's credentials for an expired token", wrongSecret(), -time.Minute, 1, true},
		{"OAuth no longer set up", unconfigured, time.Minute, 0, false},
		{"OAuth no longer set up, for an expired token", unconfigured, -time.Minute, 0, false},
	} {
		expiresIn(t, env, ada, c.expires)
		linked, before := stored(t, env), tokenRequests(env)
		got, err := c.credentials.Token(ctx, ada)
		if c.unavailable && (!errors.Is(err, github.ErrUnavailable) || got != "") {
			t.Fatalf("%s: Token = %q, %v; want ErrUnavailable", c.name, got, err)
		}
		if !c.unavailable && (err != nil || got != token) {
			t.Fatalf("%s: Token = %q, %v; want the stored token", c.name, got, err)
		}
		if n := tokenRequests(env) - before; n != c.requests {
			t.Fatalf("%s: %d refresh requests, want %d", c.name, n, c.requests)
		}
		if after := stored(t, env); len(after) != 1 || after[0] != linked[0] {
			t.Fatalf("%s: github_links = %+v, want the link as it was, %+v", c.name, after, linked)
		}
	}
}

func TestTokenWhenGitHubIsDown(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusTooManyRequests} {
		env := apptest.New(t)
		_, ada, token := expiringLink(t, env, "ada")
		ctx := context.Background()
		env.GitHub.Fail("POST /login/oauth/access_token", status)
		read := func(what string, c *github.Credentials, requests int) (string, error) {
			t.Helper()
			before := tokenRequests(env)
			got, err := c.Token(ctx, ada)
			if n := tokenRequests(env) - before; n != requests {
				t.Fatalf("HTTP %d, %s: %d refresh requests, want %d", status, what, n, requests)
			}
			return got, err
		}

		expiresIn(t, env, ada, time.Minute)
		if got, err := read("token valid for a minute", env.Credentials, 1); err != nil || got != token {
			t.Fatalf("HTTP %d, token valid for a minute: Token = %q, %v; want the stored token", status, got, err)
		}
		if got, err := read("again within 30 seconds", env.Credentials, 0); err != nil || got != token {
			t.Fatalf("HTTP %d, again within 30 seconds: Token = %q, %v; want the stored token", status, got, err)
		}
		expiresIn(t, env, ada, -time.Minute)
		linked := stored(t, env)
		if got, err := read("token expired, within 30 seconds", env.Credentials, 0); !errors.Is(err, github.ErrUnavailable) || got != "" {
			t.Fatalf("HTTP %d, token expired: Token = %q, %v; want ErrUnavailable", status, got, err)
		}
		got, err := read("token expired, by another instance", credentials(env, env.DB), 1)
		if !errors.Is(err, github.ErrUnavailable) || got != "" {
			t.Fatalf("HTTP %d, token expired, by another instance: Token = %q, %v; want ErrUnavailable", status, got, err)
		}
		if ghErr := (*github.Error)(nil); errors.As(err, &ghErr) {
			t.Fatalf("HTTP %d: Token's error wraps GitHub's %v, which callers would take for their own call's", status, ghErr)
		}
		if after := stored(t, env); len(after) != 1 || after[0] != linked[0] {
			t.Fatalf("HTTP %d: github_links = %+v, want the link as it was, %+v", status, after, linked)
		}

		env.GitHub.Fail("POST /login/oauth/access_token", 0)
		if _, err := read("GitHub back, within 30 seconds", env.Credentials, 0); !errors.Is(err, github.ErrUnavailable) {
			t.Fatalf("HTTP %d, then GitHub is back, within 30 seconds: err = %v; want ErrUnavailable", status, err)
		}
		got, err = read("GitHub back, by another instance", credentials(env, env.DB), 1)
		if err != nil || got == token || stored(t, env)[0].Token != got {
			t.Fatalf("HTTP %d, then GitHub is back: Token = %q, %v; want a new token, stored", status, got, err)
		}
		if again, err := read("after the refresh", env.Credentials, 0); err != nil || again != got {
			t.Fatalf("HTTP %d, after another instance refreshed: Token = %q, %v; want its token", status, again, err)
		}
	}
}

func TestTokenRefreshesAfterRelinkingWhileBackingOff(t *testing.T) {
	env := apptest.New(t)
	session, ada, _ := expiringLink(t, env, "ada")
	ctx := context.Background()
	expiresIn(t, env, ada, time.Minute)
	env.GitHub.Fail("POST /login/oauth/access_token", http.StatusServiceUnavailable)
	if _, err := env.Credentials.Token(ctx, ada); err != nil {
		t.Fatal(err)
	}
	env.GitHub.Fail("POST /login/oauth/access_token", 0)

	token := env.LinkGitHubOAuth(session, "ada-gh")
	expiresIn(t, env, ada, time.Minute)
	if got, err := env.Credentials.Token(ctx, ada); err != nil || got == token || env.GitHub.Refreshes() != 1 {
		t.Fatalf("Token after relinking = %q, %v after %d refreshes; want the new link's token refreshed", got, err, env.GitHub.Refreshes())
	}
}

func TestReplaced(t *testing.T) {
	env := apptest.New(t)
	_, ada, token := expiringLink(t, env, "ada")
	bob := userID(t, env, env.SignUp("bob@example.com", "correct horse"))
	ctx := context.Background()

	if env.Credentials.Replaced(ctx, ada, token) {
		t.Fatal("Replaced the token in use")
	}
	if env.Credentials.Replaced(ctx, bob, "gho_any") {
		t.Fatal("Replaced for an unlinked user")
	}
	// Another request refreshes the token.
	expiresIn(t, env, ada, time.Minute)
	refreshed, err := env.Credentials.Token(ctx, ada)
	if err != nil || refreshed == token {
		t.Fatalf("Token = %q, %v; want a new token", refreshed, err)
	}
	if !env.Credentials.Replaced(ctx, ada, token) || env.Credentials.Replaced(ctx, ada, refreshed) {
		t.Fatal("Replaced does not tell the old token from the new one")
	}
	// The token in use is due for a refresh: Replaced refreshes it, so one GitHub refused for expiring is replaced.
	expiresIn(t, env, ada, time.Minute)
	if !env.Credentials.Replaced(ctx, ada, refreshed) || env.GitHub.Refreshes() != 2 {
		t.Fatalf("Replaced a token due for a refresh: false after %d refreshes; want true after 2", env.GitHub.Refreshes())
	}
}

func TestCredentialsFail(t *testing.T) {
	env := apptest.New(t)
	adaSession, ada, token := expiringLink(t, env, "ada")
	bob := userID(t, env, env.SignUp("bob@example.com", "correct horse"))
	ctx := context.Background()
	answer := func(userID int64, token string, err error) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(ctx, "GET", "/api/projects/1/assignees", nil)
		env.Credentials.Fail(w, r, userID, token, err)
		return w
	}
	expect := func(name string, w *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		var body struct{ Error struct{ Code string } }
		json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != status || body.Error.Code != code {
			t.Fatalf("%s: answered %d %s, want %d %s", name, w.Code, body.Error.Code, status, code)
		}
	}
	unauthorized := fmt.Errorf("list assignees: %w", &github.Error{Status: http.StatusUnauthorized})

	expect("a 401 for the token in use", answer(ada, token, unauthorized), http.StatusForbidden, "github_not_linked")
	expect("a 401 for an unlinked user", answer(bob, "gho_any", unauthorized), http.StatusForbidden, "github_not_linked")
	expect("a 404", answer(ada, "gho_old", &github.Error{Status: http.StatusNotFound}), http.StatusForbidden, "forbidden")
	expect("a 5xx", answer(ada, token, &github.Error{Status: http.StatusBadGateway}), http.StatusBadGateway, "github_error")

	env.LinkGitHubOAuth(adaSession, "ada-gh") // a new token, as a refresh or a new link stores
	logs := captureLog(t)
	expect("a 401 for a token since replaced", answer(ada, token, unauthorized), http.StatusBadGateway, "github_error")
	if !strings.Contains(logs.String(), "since replaced") {
		t.Fatalf("the log does not say why:\n%s", logs)
	}
}
