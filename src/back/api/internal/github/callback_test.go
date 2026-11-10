package github_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/apptest"
)

// startLink begins linking with OAuth and returns the state cookie the browser brings back to the callback.
func startLink(t *testing.T, env *apptest.Env, session *http.Cookie) *http.Cookie {
	t.Helper()
	c := env.Do("GET", "/api/github/link", nil, session).Cookie("github_state")
	if c == nil {
		t.Fatal("link set no state cookie")
	}
	return c
}

// expectSettings fails the test unless res sends the browser to Settings with this outcome and deletes the state and
// next cookies.
func expectSettings(t *testing.T, res *apptest.Response, outcome string) {
	t.Helper()
	res.Expect(http.StatusFound, "")
	if loc, want := res.Header.Get("Location"), apptest.PublicURL+"/settings?github="+outcome+"#github"; loc != want {
		t.Fatalf("redirect to %s, want %s", loc, want)
	}
	expectFlowCookiesDeleted(t, res)
}

// expectFlowCookiesDeleted fails the test unless res deletes the cookies a flow carries to the callback.
func expectFlowCookiesDeleted(t *testing.T, res *apptest.Response) {
	t.Helper()
	for _, name := range []string{"github_state", "github_next"} {
		if c := res.Cookie(name); c == nil || c.MaxAge >= 0 || c.Path != "/api/github/callback" {
			t.Fatalf("%s cookie = %+v, want it deleted", name, c)
		}
	}
}

// linkCallback finishes linking the session's user with OAuth to the fake GitHub account login, whose new token it
// returns with the response.
func linkCallback(t *testing.T, env *apptest.Env, session *http.Cookie, login string) (*apptest.Response, string) {
	t.Helper()
	token, code := "gho_"+rand.Text(), rand.Text()
	env.GitHub.AddUser(token, apptest.GitHubID(login), login)
	env.GitHub.AddCode(code, token)
	state := startLink(t, env, session)
	return env.Do("GET", "/api/github/callback?code="+code+"&state="+url.QueryEscape(state.Value), nil, session, state), token
}

// logBuffer collects what the API logs.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// captureLog collects what the API logs until the test ends.
func captureLog(t *testing.T) *logBuffer {
	l := &logBuffer{}
	prev := log.Writer()
	log.SetOutput(l)
	t.Cleanup(func() { log.SetOutput(prev) })
	return l
}

func TestCallback(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.AddUser("gho_oauth_token", 42, "octocat")
	env.GitHub.AddCode("good-code", "gho_oauth_token")
	state := startLink(t, env, session)

	res := env.Do("GET", "/api/github/callback?code=good-code&state="+state.Value, nil, session, state)
	expectSettings(t, res, "linked")
	links := stored(t, env)
	if len(links) != 1 || links[0] != (storedLink{GitHubID: 42, Login: "octocat", Kind: "oauth", Token: "gho_oauth_token"}) {
		t.Fatalf("github_links = %+v, want one oauth link to octocat", links)
	}
}

// A GitHub account another user is linked to with OAuth signs in as them, so linking it too would leave sign-in two
// users to choose from.
func TestCallbackTaken(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	adaToken := env.LinkGitHubOAuth(ada, "octocat")
	bobToken := env.LinkGitHub(bob, "bob-gh")

	res, _ := linkCallback(t, env, bob, "octocat")
	expectSettings(t, res, "taken")
	if links := stored(t, env); len(links) != 2 || links[0].Token != adaToken || links[1].Token != bobToken {
		t.Fatalf("github_links = %+v, want both links untouched", links)
	}
	// Ada relinking it is no one else's.
	res, token := linkCallback(t, env, ada, "octocat")
	expectSettings(t, res, "linked")
	if links := stored(t, env); links[0].Token != token {
		t.Fatalf("github_links = %+v, want ada's link replaced", links)
	}
}

// A token link signs no one in, so another user's does not stop an OAuth link of the same GitHub account.
func TestCallbackIgnoresTokenLinks(t *testing.T) {
	env := apptest.New(t)
	mallory := env.SignUp("mallory@example.com", "correct horse")
	ada := env.SignUp("ada@example.com", "correct horse")
	patToken := env.LinkGitHub(mallory, "octocat")

	res, token := linkCallback(t, env, ada, "octocat")
	expectSettings(t, res, "linked")
	links := stored(t, env)
	if len(links) != 2 || links[0].Kind != "pat" || links[0].Token != patToken || links[1].Kind != "oauth" || links[1].Token != token {
		t.Fatalf("github_links = %+v, want mallory's token link untouched and ada linked with OAuth", links)
	}
}

// A user with no password signs in only with the GitHub account they are linked to, so they cannot link another
// until they set one. Relinking the same one, as to mend an expired link, works.
func TestCallbackPasswordRequired(t *testing.T) {
	env := apptest.New(t)
	session, token := env.SignInWithGitHub("octocat", "octo@example.com")

	res, _ := linkCallback(t, env, session, "other-gh")
	expectSettings(t, res, "password_required")
	if links := stored(t, env); len(links) != 1 || links[0].Login != "octocat" || links[0].Token != token {
		t.Fatalf("github_links = %+v, want the link untouched", links)
	}

	res, token = linkCallback(t, env, session, "octocat")
	expectSettings(t, res, "linked")
	if links := stored(t, env); len(links) != 1 || links[0].Login != "octocat" || links[0].Kind != "oauth" || links[0].Token != token {
		t.Fatalf("github_links = %+v, want the link replaced by the new OAuth token", links)
	}

	env.Do("PATCH", "/api/me/password", map[string]string{"newPassword": "correct horse"}, session).Expect(http.StatusNoContent, "")
	res, token = linkCallback(t, env, session, "other-gh")
	expectSettings(t, res, "linked")
	if links := stored(t, env); len(links) != 1 || links[0].Login != "other-gh" || links[0].Token != token {
		t.Fatalf("github_links = %+v, want the link to other-gh", links)
	}
}

// One GitHub account's links and sign-ins take turns under the sign-in's lock, so at once they still link it to one
// user.
func TestCallbackConcurrent(t *testing.T) {
	env := apptest.New(t)
	ctx := context.Background()
	id := apptest.GitHubID("octocat")
	type callback struct {
		path    string
		cookies []*http.Cookie
	}
	var calls []callback
	for _, email := range []string{"ada@example.com", "bob@example.com"} {
		session := env.SignUp(email, "correct horse")
		token, code := "gho_"+rand.Text(), rand.Text()
		env.GitHub.AddUser(token, id, "octocat")
		env.GitHub.AddCode(code, token)
		state := startLink(t, env, session)
		calls = append(calls, callback{"/api/github/callback?code=" + code + "&state=" + url.QueryEscape(state.Value), []*http.Cookie{session, state}})
	}
	code, _ := gitHubAccount(env, "octocat", verified("octo@example.com"))
	state := startSignIn(t, env)
	calls = append(calls, callback{"/api/github/callback?code=" + code + "&state=" + url.QueryEscape(state.Value), []*http.Cookie{state}})

	// Hold the lock, so all three wait for it.
	conn, err := env.DB.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	var pid int
	if err := conn.QueryRow(ctx, `select pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `select pg_advisory_lock(hashtext('github sign-in'), hashtext($1::bigint::text))`, id); err != nil {
		t.Fatal(err)
	}
	unlock := sync.OnceFunc(func() {
		conn.Exec(ctx, `select pg_advisory_unlock(hashtext('github sign-in'), hashtext($1::bigint::text))`, id)
	})
	defer unlock()
	results := make([]struct {
		location string
		session  *http.Cookie
		err      error
	}, len(calls))
	var wg sync.WaitGroup
	for i, c := range calls {
		wg.Go(func() {
			res, err := get(env, c.path, c.cookies...)
			if err != nil {
				results[i].err = err
				return
			}
			results[i].location = res.Header.Get("Location")
			for _, ck := range res.Cookies() {
				if ck.Name == "session" && ck.MaxAge > 0 {
					results[i].session = ck
				}
			}
		})
	}
	for waiting, deadline := 0, time.Now().Add(5*time.Second); waiting < len(calls); {
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d callbacks waited for the GitHub account's lock", waiting, len(calls))
		}
		time.Sleep(10 * time.Millisecond)
		// On the lock's connection: the waiting callbacks may hold all the others the pool has.
		if err := conn.QueryRow(ctx, `select count(*) from pg_stat_activity where $1 = any(pg_blocking_pids(pid))`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
	}
	unlock()
	wg.Wait()

	var linked []int // of the links
	for i, r := range results[:2] {
		switch r.location {
		case apptest.PublicURL + "/settings?github=linked#github":
			linked = append(linked, i)
		case apptest.PublicURL + "/settings?github=taken#github":
		default:
			t.Fatalf("link %d: redirect to %q, error %v; want linked or taken", i, r.location, r.err)
		}
	}
	signedIn := results[2]
	if signedIn.err != nil || signedIn.location != apptest.PublicURL+"/" || signedIn.session == nil {
		t.Fatalf("sign-in: redirect to %q, session %v, error %v; want home, signed in", signedIn.location, signedIn.session, signedIn.err)
	}
	rows, err := env.DB.Query(ctx, `select user_id from github_links where github_id = $1 and kind = 'oauth'`, id)
	if err != nil {
		t.Fatal(err)
	}
	owners, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		t.Fatal(err)
	}
	signedInAs := getMe(t, env, signedIn.session).ID
	if len(owners) != 1 || len(linked) > 1 || signedInAs != owners[0] {
		t.Fatalf("links %v worked, users %v are linked to octocat with OAuth, the sign-in got user %d; want one user, the one signed in",
			linked, owners, signedInAs)
	}
	// The sign-in made a user only if it went first, and then no link worked.
	if len(linked) == 1 && owners[0] != userID(t, env, calls[linked[0]].cookies[0]) {
		t.Fatalf("link %d worked, but user %d is linked to octocat", linked[0], owners[0])
	}
	if u := users(t, env); len(u) != 3-len(linked) {
		t.Fatalf("users = %q, want ada and bob, and a new user only when no link worked", u)
	}
}

func TestCallbackStoresAnExpiringToken(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.ExpireTokens(8 * time.Hour)
	token := env.LinkGitHubOAuth(session, "octocat")

	links := stored(t, env)
	if len(links) != 1 || links[0].Token != token || links[0].Refresh == "" || time.Until(links[0].Expires) < 8*time.Hour-time.Minute {
		t.Fatalf("github_links = %+v, want the token, its refresh token and an expiry 8 hours away", links)
	}
	// Relinking with a token that does not expire forgets the old one's refresh token and expiry.
	env.GitHub.ExpireTokens(0)
	token = env.LinkGitHubOAuth(session, "octocat")
	if links := stored(t, env); len(links) != 1 || links[0].Token != token || links[0].Refresh != "" || !links[0].Expires.IsZero() {
		t.Fatalf("github_links = %+v, want the new token, with no refresh token or expiry", links)
	}
}

func TestCallbackFailuresStoreNothing(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.AddUser("gho_oauth_token", 42, "octocat")
	cases := []struct {
		name   string
		query  string // STATE stands for the state link issued
		cookie string // the github_state the browser sends: "link" for the one link set, "none", or "empty"
	}{
		{"wrong state", "code=good-code&state=forged", "link"},
		{"missing state", "code=good-code", "link"},
		{"missing cookie", "code=good-code&state=STATE", "none"},
		{"empty state and cookie", "code=good-code&state=", "empty"},
		{"GitHub's error", "error=redirect_uri_mismatch&error_description=The+redirect_uri+MUST+match.&state=STATE", "link"},
		{"GitHub's error without state", "error=application_suspended", "none"},
		{"bad code", "code=bad-code&state=STATE", "link"},
		{"token GitHub rejects", "code=orphan-code&state=STATE", "link"},
	}
	logs := captureLog(t)
	for _, c := range cases {
		env.GitHub.AddCode("good-code", "gho_oauth_token")
		env.GitHub.AddCode("orphan-code", "gho_unknown")
		state := startLink(t, env, session)
		cookies := []*http.Cookie{session}
		switch c.cookie {
		case "link":
			cookies = append(cookies, state)
		case "empty":
			cookies = append(cookies, &http.Cookie{Name: "github_state", Value: ""})
		}

		// Outside t.Run, because env.Do fails the parent test.
		res := env.Do("GET", "/api/github/callback?"+strings.ReplaceAll(c.query, "STATE", state.Value), nil, cookies...)
		t.Run(c.name, func(t *testing.T) {
			expectSettings(t, res, "error")
			if links := stored(t, env); len(links) != 0 {
				t.Fatalf("github_links = %+v, want none", links)
			}
		})
	}
	for _, secret := range []string{"good-code", "bad-code", "orphan-code", "gho_oauth_token", "gho_unknown", env.Config.GitHubClientSecret} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("the log has %q:\n%s", secret, logs)
		}
	}
}

func TestCallbackLogsWhy(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	logs := captureLog(t)
	for _, c := range []struct {
		query string
		want  []string
	}{
		{"error=redirect_uri_mismatch&error_description=The+redirect_uri+MUST+match+the+registered+callback+URL.",
			[]string{`"redirect_uri_mismatch": "The redirect_uri MUST match the registered callback URL."`, apptest.PublicURL + "/api/github/callback"}},
		// The token endpoint's refusal.
		{"code=bad-code", []string{`"bad_verification_code": "The code passed is incorrect or expired."`}},
		// Anyone can send the browser here with any query: quoted, a newline in it forges no log line.
		{"error=e%0A2026/11/09+00:00:00+forged&error_description=d%0Aforged+too", []string{`"e\n2026/11/09 00:00:00 forged": "d\nforged too"`}},
	} {
		state := startLink(t, env, session)
		res := env.Do("GET", "/api/github/callback?"+c.query+"&state="+state.Value, nil, session, state)
		expectSettings(t, res, "error")
		for _, want := range c.want {
			if !strings.Contains(logs.String(), want) {
				t.Errorf("the log does not have %q:\n%s", want, logs)
			}
		}
	}
	if strings.Contains(logs.String(), "\nforged") || strings.Contains(logs.String(), "\n2026/11/09 00:00:00 forged") {
		t.Errorf("the callback's query forged a log line:\n%s", logs)
	}
}

func TestCallbackDenied(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	token := env.LinkGitHub(session, "octocat")
	env.GitHub.AddCode("good-code", "gho_oauth_token")
	state := startLink(t, env, session)

	res := env.Do("GET", "/api/github/callback?error=access_denied&error_description=The+user+has+denied+your+application+access.&state="+
		state.Value, nil, session, state)
	expectSettings(t, res, "denied")
	if links := stored(t, env); len(links) != 1 || links[0].Token != token {
		t.Fatalf("github_links = %+v, want the earlier link untouched", links)
	}
}

// Only a valid state shows GitHub sent the browser back for this session, so error means nothing without one.
func TestCallbackDeniedNeedsItsState(t *testing.T) {
	env := apptest.New(t)
	other := env.SignUp("mallory@example.com", "correct horse")
	session := env.SignUp("ada@example.com", "correct horse")
	logs := captureLog(t)
	for _, c := range []struct {
		name   string
		state  func() (query string, cookie *http.Cookie)
		reason string
	}{
		{"wrong state", func() (string, *http.Cookie) { return "forged", startLink(t, env, session) }, "does not match"},
		{"no state", func() (string, *http.Cookie) { return "", startLink(t, env, session) }, "does not match"},
		{"no cookie", func() (string, *http.Cookie) { return startLink(t, env, session).Value, nil }, "does not match"},
		{"another session's state", func() (string, *http.Cookie) { c := startLink(t, env, other); return c.Value, c }, "not tied to this session"},
	} {
		query, cookie := c.state()
		cookies := []*http.Cookie{session}
		if cookie != nil {
			cookies = append(cookies, cookie)
		}
		// Outside t.Run, because env.Do fails the parent test.
		res := env.Do("GET", "/api/github/callback?error=access_denied&state="+query, nil, cookies...)
		t.Run(c.name, func(t *testing.T) {
			expectSettings(t, res, "error")
			if !strings.Contains(logs.String(), c.reason) || !strings.Contains(logs.String(), `the callback also had error "access_denied"`) {
				t.Fatalf("the log does not say the state was wrong and what error came with it:\n%s", logs)
			}
		})
	}
}

func TestCallbackRejectsStateNotFromThisSession(t *testing.T) {
	env := apptest.New(t)
	attacker := env.SignUp("mallory@example.com", "correct horse")
	victim := env.SignUp("ada@example.com", "correct horse")
	env.GitHub.AddUser("gho_attacker", 666, "mallory-gh")
	for name, state := range map[string]*http.Cookie{
		"another session's state": startLink(t, env, attacker),
		"planted state":           {Name: "github_state", Value: "X"},
	} {
		env.GitHub.AddCode("attacker-code", "gho_attacker")
		// Outside t.Run, because env.Do fails the parent test.
		res := env.Do("GET", "/api/github/callback?code=attacker-code&state="+state.Value, nil, victim, state)
		t.Run(name, func(t *testing.T) {
			expectSettings(t, res, "error")
			if links := stored(t, env); len(links) != 0 {
				t.Fatalf("github_links = %+v, want none", links)
			}
		})
	}
}

func TestCallbackSignedOut(t *testing.T) {
	env := apptest.New(t)
	logs := captureLog(t)
	for name, session := range map[string]*http.Cookie{
		"no session":      nil,
		"expired session": {Name: "session", Value: "expired"},
	} {
		env.GitHub.AddCode("good-code", "gho_oauth_token")
		// Outside t.Run, because env.Do fails the parent test.
		res := env.Do("GET", "/api/github/callback?code=good-code&state=x", nil, session, &http.Cookie{Name: "github_state", Value: "x"})
		t.Run(name, func(t *testing.T) {
			expectSettings(t, res, "error")
			if want := "the browser must be signed in at " + apptest.PublicURL; !strings.Contains(logs.String(), want) {
				t.Fatalf("the log does not have %q:\n%s", want, logs)
			}
		})
	}
	if links := stored(t, env); len(links) != 0 {
		t.Fatalf("github_links = %+v, want none", links)
	}
}
