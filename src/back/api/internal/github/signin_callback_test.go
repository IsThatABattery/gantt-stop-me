package github_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
)

// verified is a GitHub account's verified primary email.
func verified(email string) githubtest.Email {
	return githubtest.Email{Email: email, Primary: true, Verified: true}
}

// gitHubAccount adds the fake GitHub account login, with emails when there are any, and a code that exchanges for
// a new token of it, which it returns with the code.
func gitHubAccount(env *apptest.Env, login string, emails ...githubtest.Email) (code, token string) {
	code, token = rand.Text(), "gho_"+rand.Text()
	env.GitHub.AddUser(token, apptest.GitHubID(login), login)
	if emails != nil {
		env.GitHub.SetEmails(apptest.GitHubID(login), emails...)
	}
	env.GitHub.AddCode(code, token)
	return code, token
}

// startSignIn begins signing in with GitHub and returns the state cookie the browser brings back to the callback.
func startSignIn(t *testing.T, env *apptest.Env) *http.Cookie {
	t.Helper()
	c := env.Do("GET", "/api/auth/github", nil).Cookie("github_state")
	if c == nil {
		t.Fatal("sign-in set no state cookie")
	}
	return c
}

// signIn signs in with GitHub, as GitHub sends the browser back with code, with cookies besides the state's.
func signIn(t *testing.T, env *apptest.Env, code string, cookies ...*http.Cookie) *apptest.Response {
	t.Helper()
	state := startSignIn(t, env)
	return env.Do("GET", "/api/github/callback?code="+code+"&state="+url.QueryEscape(state.Value), nil, append(cookies, state)...)
}

// expectHome fails the test unless res sends the browser home, deletes the state and next cookies and starts a
// session, which it returns.
func expectHome(t *testing.T, res *apptest.Response) *http.Cookie {
	t.Helper()
	return expectSignedIn(t, res, "/")
}

// expectSignedIn is expectHome for a sign-in that sends the browser to path.
func expectSignedIn(t *testing.T, res *apptest.Response, path string) *http.Cookie {
	t.Helper()
	res.Expect(http.StatusFound, "")
	if loc, want := res.Header.Get("Location"), apptest.PublicURL+path; loc != want {
		t.Fatalf("redirect to %s, want %s", loc, want)
	}
	expectFlowCookiesDeleted(t, res)
	session := res.Cookie("session")
	if session == nil || session.MaxAge != 30*24*60*60 || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode || session.Path != "/" {
		t.Fatalf("session cookie = %+v, want one as logging in sets", session)
	}
	return session
}

// expectLogin fails the test unless res sends the browser back to sign-in with this outcome, deletes the state and
// next cookies and starts no session.
func expectLogin(t *testing.T, res *apptest.Response, outcome string) {
	t.Helper()
	res.Expect(http.StatusFound, "")
	if loc, want := res.Header.Get("Location"), apptest.PublicURL+"/login?github="+outcome; loc != want {
		t.Fatalf("redirect to %s, want %s", loc, want)
	}
	expectFlowCookiesDeleted(t, res)
	if c := res.Cookie("session"); c != nil {
		t.Fatalf("set the session cookie %+v, want none", c)
	}
}

// me is who session signs in as.
type me struct {
	ID          int64
	Email       string
	HasPassword bool
	GitHub      *struct{ Login, Kind string }
}

func getMe(t *testing.T, env *apptest.Env, session *http.Cookie) me {
	t.Helper()
	res := env.Do("GET", "/api/me", nil, session)
	res.Expect(http.StatusOK, "")
	var m me
	res.JSON(&m)
	return m
}

// users lists every user's email, oldest first, marked "(no password)" for a user with none.
func users(t *testing.T, env *apptest.Env) []string {
	t.Helper()
	rows, err := env.DB.Query(context.Background(),
		`select email::text || case when password_hash is null then ' (no password)' else '' end from users order by id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var list []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		list = append(list, s)
	}
	return list
}

func TestSignInCallbackMakesAUser(t *testing.T) {
	env := apptest.New(t)
	env.GitHub.ExpireTokens(8 * time.Hour)
	code, token := gitHubAccount(env, "octocat",
		githubtest.Email{Email: "123+octocat@users.noreply.github.com", Verified: true},
		githubtest.Email{Email: "work@example.com", Primary: false, Verified: true},
		verified("octo@example.com"))

	session := expectHome(t, signIn(t, env, code))
	got := getMe(t, env, session)
	if got.Email != "octo@example.com" || got.HasPassword || got.GitHub == nil || *got.GitHub != (struct{ Login, Kind string }{"octocat", "oauth"}) {
		t.Fatalf("me = %+v, want octo@example.com with no password, linked to octocat with OAuth", got)
	}
	if u := users(t, env); !slices.Equal(u, []string{"octo@example.com (no password)"}) {
		t.Fatalf("users = %q, want only the new one", u)
	}
	links := stored(t, env)
	if len(links) != 1 || links[0].GitHubID != apptest.GitHubID("octocat") || links[0].Kind != "oauth" || links[0].Token != token ||
		links[0].Refresh == "" || links[0].Expires.IsZero() {
		t.Fatalf("github_links = %+v, want the new user's OAuth link with its refresh token and expiry", links)
	}
	// Signing in again finds them.
	code, _ = gitHubAccount(env, "octocat")
	if again := getMe(t, env, expectHome(t, signIn(t, env, code))); again.ID != got.ID {
		t.Fatalf("signing in again got user %d, want %d", again.ID, got.ID)
	}
	if u := users(t, env); len(u) != 1 {
		t.Fatalf("users = %q, want one", u)
	}
}

func TestSignInCallbackSignsInTheLinkedUser(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	env.LinkGitHubOAuth(ada, "octocat")
	addMembership(t, env, ada)
	code, token := gitHubAccount(env, "octocat", verified("octo@example.com"))

	session := expectHome(t, signIn(t, env, code))
	if got, want := getMe(t, env, session), getMe(t, env, ada); got.ID != want.ID || !got.HasPassword {
		t.Fatalf("signed in as %+v, want ada, %+v", got, want)
	}
	// The link is replaced, as relinking does.
	if links := stored(t, env); len(links) != 1 || links[0].Kind != "oauth" || links[0].Token != token {
		t.Fatalf("github_links = %+v, want ada's link replaced by the new OAuth token", links)
	}
	var stale bool
	if err := env.DB.QueryRow(context.Background(), `select bool_and(checked_at = '-infinity') from project_members`).Scan(&stale); err != nil {
		t.Fatal(err)
	}
	if n := memberships(t, env, ada); n != 1 || !stale {
		t.Fatalf("ada has %d project memberships, stale %v; want hers kept, to be checked again", n, stale)
	}
	if u := users(t, env); !slices.Equal(u, []string{"ada@example.com"}) {
		t.Fatalf("users = %q, want only ada", u)
	}
	for _, req := range env.GitHub.Requests() {
		if strings.Contains(req, "/user/emails") {
			t.Fatalf("asked GitHub %s, which only a new user needs", req)
		}
	}
	env.Do("GET", "/api/me", nil, ada).Expect(http.StatusOK, "") // her other sessions stay
}

func TestSignInCallbackWhileSignedIn(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	env.LinkGitHubOAuth(ada, "octocat")
	bob := env.SignUp("bob@example.com", "correct horse")
	code, _ := gitHubAccount(env, "octocat")

	session := expectHome(t, signIn(t, env, code, bob))
	if session.Value == bob.Value || getMe(t, env, session).ID != getMe(t, env, ada).ID {
		t.Fatal("a browser signed in as bob did not get a session for ada")
	}
}

// Settings once linked any GitHub account, even one another user had linked, and links from then remain, so
// sign-in cannot tell which user it is.
func TestSignInCallbackAmbiguous(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	adaToken, bobToken := env.LinkGitHubOAuth(ada, "octocat"), env.LinkGitHubOAuth(bob, "bob-gh")
	if _, err := env.DB.Exec(context.Background(), `update github_links set github_id = $1, login = 'octocat' where user_id = $2`,
		apptest.GitHubID("octocat"), userID(t, env, bob)); err != nil {
		t.Fatal(err)
	}
	code, _ := gitHubAccount(env, "octocat", verified("octo@example.com"))
	logs := captureLog(t)

	expectLogin(t, signIn(t, env, code), "ambiguous")
	if links := stored(t, env); len(links) != 2 || links[0].Token != adaToken || links[1].Token != bobToken {
		t.Fatalf("github_links = %+v, want both links untouched", links)
	}
	if u := users(t, env); len(u) != 2 {
		t.Fatalf("users = %q, want ada and bob only", u)
	}
	if want := fmt.Sprintf(`more than one user is linked to the GitHub account: GitHub account %d, "octocat"`, apptest.GitHubID("octocat")); !strings.Contains(logs.String(), want) {
		t.Fatalf("the log does not have %q:\n%s", want, logs)
	}
}

// Signing in as a user who has the GitHub account's email would let whoever holds that GitHub account into theirs.
func TestSignInCallbackAccountExists(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	env.LinkGitHub(ada, "ada-gh") // another GitHub account than the one signing in
	code, _ := gitHubAccount(env, "octocat", verified("ADA@Example.com"))

	expectLogin(t, signIn(t, env, code), "account_exists")
	if u := users(t, env); !slices.Equal(u, []string{"ada@example.com"}) {
		t.Fatalf("users = %q, want only ada", u)
	}
	if links := stored(t, env); len(links) != 1 || links[0].Login != "ada-gh" {
		t.Fatalf("github_links = %+v, want ada's link untouched", links)
	}
}

// Any token of a GitHub account links it with a personal access token, such as one a third-party app was given, so
// such a link must not sign the account's owner in to the linker's account, or hand it their new token.
func TestSignInCallbackIgnoresTokenLinks(t *testing.T) {
	env := apptest.New(t)
	mallory := env.SignUp("mallory@example.com", "correct horse")
	patToken := env.LinkGitHub(mallory, "octocat")
	code, token := gitHubAccount(env, "octocat", verified("octo@example.com"))

	session := expectHome(t, signIn(t, env, code))
	if got := getMe(t, env, session); got.ID == getMe(t, env, mallory).ID || got.Email != "octo@example.com" {
		t.Fatalf("signed in as %+v, want a new user for octocat, not mallory", got)
	}
	links := stored(t, env)
	if len(links) != 2 || links[0].Kind != "pat" || links[0].Token != patToken || links[1].Kind != "oauth" || links[1].Token != token {
		t.Fatalf("github_links = %+v, want mallory's token link untouched and the new user linked with OAuth", links)
	}
}

// The sign-in's start remembers a path on the app to send the browser to, and anything else sends it home.
func TestSignInCallbackNext(t *testing.T) {
	env := apptest.New(t)
	for _, c := range []struct{ next, want string }{
		{"/settings", "/settings"},
		{"/projects/12", "/projects/12"},
		{"/", "/"},
		{"", "/"},
		{"//evil.example", "/"},
		{"//evil.example/settings", "/"},
		{"https://evil.example", "/"},
		{"/\\evil.example", "/"},
		{"/settings?github=linked", "/"},
		{"/settings#github", "/"},
		{"/../settings", "/"},
		{"settings", "/"},
	} {
		code, _ := gitHubAccount(env, "octocat", verified("octo@example.com"))
		start := env.Do("GET", "/api/auth/github?next="+url.QueryEscape(c.next), nil)
		state, next := start.Cookie("github_state"), start.Cookie("github_next")
		cookies := []*http.Cookie{state}
		if next != nil && next.MaxAge > 0 {
			cookies = append(cookies, next)
		}
		// Outside t.Run, because env.Do fails the parent test.
		res := env.Do("GET", "/api/github/callback?code="+code+"&state="+url.QueryEscape(state.Value), nil, cookies...)
		t.Run(c.next, func(t *testing.T) {
			if c.want == c.next {
				if next == nil || next.Value != c.next || !next.HttpOnly || next.SameSite != http.SameSiteLaxMode || next.Secure ||
					next.Path != "/api/github/callback" || next.MaxAge != 600 {
					t.Fatalf("next cookie = %+v, want %q set as the state cookie is", next, c.next)
				}
			} else if next == nil || next.MaxAge >= 0 {
				t.Fatalf("next cookie = %+v, want it deleted, so an earlier sign-in's does not apply", next)
			}
			expectSignedIn(t, res, c.want)
		})
	}
}

// A browser can send the callback any github_next, so it counts only as a path on the app.
func TestSignInCallbackChecksNextAgain(t *testing.T) {
	env := apptest.New(t)
	for _, next := range []string{"//evil.example", "https:", "/settings?x=1"} {
		code, _ := gitHubAccount(env, "octocat", verified("octo@example.com"))
		res := signIn(t, env, code, &http.Cookie{Name: "github_next", Value: next})
		t.Run(next, func(t *testing.T) { expectHome(t, res) })
	}
}

// A failed sign-in forgets where it would have sent the browser, as every callback does.
func TestSignInCallbackFailureClearsNext(t *testing.T) {
	env := apptest.New(t)
	start := env.Do("GET", "/api/auth/github?next=/settings", nil)
	state, next := start.Cookie("github_state"), start.Cookie("github_next")
	res := env.Do("GET", "/api/github/callback?error=access_denied&state="+url.QueryEscape(state.Value), nil, state, next)
	expectLogin(t, res, "denied")
}

func TestSignInCallbackNoEmail(t *testing.T) {
	env := apptest.New(t)
	for name, emails := range map[string][]githubtest.Email{
		"none":                 {},
		"primary not verified": {{Email: "octo@example.com", Primary: true}},
		"verified not primary": {{Email: "octo@example.com", Verified: true}, {Email: "other@example.com", Primary: true}},
	} {
		code, _ := gitHubAccount(env, "octocat", emails...)
		// Outside t.Run, because env.Do fails the parent test.
		res := signIn(t, env, code)
		t.Run(name, func(t *testing.T) {
			expectLogin(t, res, "no_email")
			if u := users(t, env); len(u) != 0 {
				t.Fatalf("users = %q, want none", u)
			}
			if links := stored(t, env); len(links) != 0 {
				t.Fatalf("github_links = %+v, want none", links)
			}
		})
	}
}

func TestSignInCallbackDenied(t *testing.T) {
	env := apptest.New(t)
	state := startSignIn(t, env)

	res := env.Do("GET", "/api/github/callback?error=access_denied&error_description=The+user+has+denied+your+application+access.&state="+
		url.QueryEscape(state.Value), nil, state)
	expectLogin(t, res, "denied")
	if u := users(t, env); len(u) != 0 {
		t.Fatalf("users = %q, want none", u)
	}
}

// Only a valid state shows GitHub sent this browser back from the sign-in it started, so nothing counts without one,
// not even GitHub's error.
func TestSignInCallbackNeedsItsState(t *testing.T) {
	env := apptest.New(t)
	logs := captureLog(t)
	for _, c := range []struct {
		name   string
		query  string // STATE stands for the state the browser's sign-in got, OTHER for another browser's
		cookie bool   // whether the browser sends its github_state
	}{
		{"forged state", "code=CODE&state=signin.forged", true},
		{"another browser's state", "code=CODE&state=OTHER", true},
		{"missing state", "code=CODE", true},
		{"empty state", "code=CODE&state=", true},
		{"missing cookie", "code=CODE&state=STATE", false},
		{"denied with a forged state", "error=access_denied&state=signin.forged", true},
		{"denied without state", "error=access_denied", true},
		{"denied without cookie", "error=access_denied&state=STATE", false},
	} {
		code, _ := gitHubAccount(env, "octocat", verified("octo@example.com"))
		state, other := startSignIn(t, env), startSignIn(t, env)
		var cookies []*http.Cookie
		if c.cookie {
			cookies = append(cookies, state)
		}
		query := strings.NewReplacer("CODE", code, "STATE", url.QueryEscape(state.Value), "OTHER", url.QueryEscape(other.Value)).Replace(c.query)
		// Outside t.Run, because env.Do fails the parent test.
		res := env.Do("GET", "/api/github/callback?"+query, nil, cookies...)
		t.Run(c.name, func(t *testing.T) {
			expectLogin(t, res, "error")
			if u := users(t, env); len(u) != 0 {
				t.Fatalf("users = %q, want none", u)
			}
			if links := stored(t, env); len(links) != 0 {
				t.Fatalf("github_links = %+v, want none", links)
			}
		})
	}
	if !strings.Contains(logs.String(), "state does not match the github_state cookie") ||
		!strings.Contains(logs.String(), `the callback also had error "access_denied"`) {
		t.Fatalf("the log does not say the state was wrong and what error came with it:\n%s", logs)
	}
}

func TestSignInCallbackFailures(t *testing.T) {
	env := apptest.New(t)
	logs := captureLog(t)
	var codes, tokens []string
	for _, c := range []struct {
		name  string
		query string // CODE stands for a good code
		fail  string // the fake's route that answers 500
		want  string // in the log
	}{
		{"bad code", "code=bad-code", "", `GitHub refused the sign-in: "bad_verification_code": "The code passed is incorrect or expired."`},
		{"GitHub's error", "error=redirect_uri_mismatch&error_description=The+redirect_uri+MUST+match.",
			"", `GitHub refused the sign-in: "redirect_uri_mismatch": "The redirect_uri MUST match."; the OAuth App's callback URL must be ` +
				apptest.PublicURL + "/api/github/callback"},
		// Anyone can send the browser here with any query: quoted, a newline in it forges no log line.
		{"forged log line", "error=e%0A2026/11/09+00:00:00+forged", "", `"e\n2026/11/09 00:00:00 forged"`},
		{"GitHub's user fails", "code=CODE", "GET /user", "load the GitHub user: github: HTTP 500"},
		{"GitHub's emails fail", "code=CODE", "GET /user/emails", "list the GitHub user's emails: github: HTTP 500"},
	} {
		code, token := gitHubAccount(env, "octocat", verified("octo@example.com"))
		codes, tokens = append(codes, code), append(tokens, token)
		if c.fail != "" {
			env.GitHub.Fail(c.fail, http.StatusInternalServerError)
		}
		state := startSignIn(t, env)
		res := env.Do("GET", "/api/github/callback?"+strings.ReplaceAll(c.query, "CODE", code)+"&state="+url.QueryEscape(state.Value), nil, state)
		if c.fail != "" {
			env.GitHub.Fail(c.fail, 0)
		}
		t.Run(c.name, func(t *testing.T) {
			expectLogin(t, res, "error")
			if !strings.Contains(logs.String(), c.want) {
				t.Fatalf("the log does not have %q:\n%s", c.want, logs)
			}
			if u := users(t, env); len(u) != 0 {
				t.Fatalf("users = %q, want none", u)
			}
		})
	}
	if strings.Contains(logs.String(), "\n2026/11/09 00:00:00 forged") {
		t.Errorf("the callback's query forged a log line:\n%s", logs)
	}
	for _, secret := range append(append(codes, tokens...), "bad-code", env.Config.GitHubClientSecret) {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("the log has %q:\n%s", secret, logs)
		}
	}
}

// Two sign-ins of one GitHub account that no user is linked to, both past the point of finding none, make one user.
func TestSignInCallbackConcurrent(t *testing.T) {
	env := apptest.New(t)
	var arrived atomic.Int32
	both := make(chan struct{})
	env.GitHub.Before("GET /user/emails", func() {
		if arrived.Add(1) == 2 {
			close(both)
		}
		select {
		case <-both:
		case <-time.After(5 * time.Second):
		}
	})
	type callback struct {
		path  string
		state *http.Cookie
	}
	var calls [2]callback
	for i := range calls {
		code, _ := gitHubAccount(env, "octocat", verified("octo@example.com"))
		state := startSignIn(t, env)
		calls[i] = callback{"/api/github/callback?code=" + code + "&state=" + url.QueryEscape(state.Value), state}
	}
	var results [2]struct {
		location string
		session  *http.Cookie
		err      error
	}
	var wg sync.WaitGroup
	for i, c := range calls {
		wg.Go(func() {
			res, err := get(env, c.path, c.state)
			if err != nil {
				results[i].err = err
				return
			}
			results[i].location = res.Header.Get("Location")
			for _, ck := range res.Cookies() {
				if ck.Name == "session" {
					results[i].session = ck
				}
			}
		})
	}
	wg.Wait()
	if n := arrived.Load(); n != 2 {
		t.Fatalf("%d of 2 sign-ins asked for the emails, want both, as neither found a linked user", n)
	}
	var ids []int64
	for i, r := range results {
		if r.err != nil || r.location != apptest.PublicURL+"/" || r.session == nil {
			t.Fatalf("sign-in %d: redirect to %q, session %v, error %v; want home, signed in", i, r.location, r.session, r.err)
		}
		ids = append(ids, getMe(t, env, r.session).ID)
	}
	if ids[0] != ids[1] {
		t.Fatalf("the sign-ins signed in as users %v, want one", ids)
	}
	if u := users(t, env); !slices.Equal(u, []string{"octo@example.com (no password)"}) {
		t.Fatalf("users = %q, want one", u)
	}
	if links := stored(t, env); len(links) != 1 {
		t.Fatalf("github_links = %+v, want one", links)
	}
}

// get is a GET for goroutines other than the test's, which returns an error instead of failing the test. It follows
// no redirect.
func get(env *apptest.Env, path string, cookies ...*http.Cookie) (*http.Response, error) {
	req, err := http.NewRequest("GET", env.Server.URL+path, nil)
	if err != nil {
		return nil, err
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	res.Body.Close()
	return res, nil
}
