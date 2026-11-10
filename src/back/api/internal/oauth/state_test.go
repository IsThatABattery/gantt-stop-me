package oauth_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/oauth"
)

var states = oauth.StateCookie{Name: "test_state", Path: "/api/test/callback"}

// issue starts a flow of kind for session, and returns its state and the cookie the browser keeps.
func issue(t *testing.T, c oauth.StateCookie, kind oauth.Kind, session string) (string, *http.Cookie) {
	t.Helper()
	w := httptest.NewRecorder()
	state := c.Issue(w, kind, session)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Issue set %d cookies, want 1", len(cookies))
	}
	return state, cookies[0]
}

// callback is the request the browser sends the callback with state in its query and cookie, if not nil.
func callback(state string, cookie *http.Cookie) *http.Request {
	r := httptest.NewRequest("GET", "/api/test/callback?code=c&state="+url.QueryEscape(state), nil)
	if cookie != nil {
		r.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	return r
}

func TestIssue(t *testing.T) {
	for _, secure := range []bool{false, true} {
		c := states
		c.Secure = secure
		for _, kind := range []oauth.Kind{oauth.Link, oauth.SignIn} {
			state, cookie := issue(t, c, kind, "session-token")
			if cookie.Name != "test_state" || cookie.Value != state || cookie.Path != "/api/test/callback" || cookie.MaxAge != 600 ||
				!cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Secure != secure {
				t.Fatalf("cookie = %+v, want test_state=%s, Path=/api/test/callback, MaxAge=600, HttpOnly, SameSite=Lax, Secure=%v",
					cookie, state, secure)
			}
			if oauth.KindOf(state) != kind {
				t.Fatalf("KindOf(%q) = %q, want %q", state, oauth.KindOf(state), kind)
			}
			if strings.Contains(state, "session-token") {
				t.Fatalf("state %q has the session token", state)
			}
			if again, _ := issue(t, c, kind, "session-token"); again == state {
				t.Fatal("two flows got the same state")
			}
		}
	}
}

func TestClear(t *testing.T) {
	w := httptest.NewRecorder()
	states.Clear(w)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "test_state" || cookies[0].Path != "/api/test/callback" || cookies[0].MaxAge >= 0 {
		t.Fatalf("Clear set %+v, want test_state deleted at /api/test/callback", cookies)
	}
}

func TestVerify(t *testing.T) {
	link, linkCookie := issue(t, states, oauth.Link, "ada-session")
	signIn, signInCookie := issue(t, states, oauth.SignIn, "")
	other, otherCookie := issue(t, states, oauth.Link, "mallory-session")
	// planted is a cookie the browser may have been given by another site under the app's domain.
	planted := func(state string) *http.Cookie { return &http.Cookie{Name: "test_state", Value: state} }
	// tampered is state with the first character of its part i changed.
	tampered := func(state string, i int) string {
		parts := strings.Split(state, ".")
		b := []byte(parts[i])
		b[0] ^= 1
		parts[i] = string(b)
		return strings.Join(parts, ".")
	}
	unbound := link[:strings.LastIndex(link, ".")]
	wrongKind := "Link" + strings.TrimPrefix(link, "link")
	cases := []struct {
		name    string
		state   string
		cookie  *http.Cookie
		kind    oauth.Kind
		session string
		want    string // in the error, or "" for none
	}{
		{"link", link, linkCookie, oauth.Link, "ada-session", ""},
		{"sign-in", signIn, signInCookie, oauth.SignIn, "", ""},

		{"no cookie", link, nil, oauth.Link, "ada-session", "does not match the test_state cookie"},
		{"no state", "", linkCookie, oauth.Link, "ada-session", "does not match"},
		{"empty state and cookie", "", planted(""), oauth.Link, "ada-session", "does not match"},
		{"another flow's cookie", link, otherCookie, oauth.Link, "ada-session", "does not match"},
		{"another flow's state", other, linkCookie, oauth.Link, "ada-session", "does not match"},
		{"sign-in without its cookie", signIn, nil, oauth.SignIn, "", "does not match"},
		{"sign-in with another cookie", signIn, linkCookie, oauth.SignIn, "", "does not match"},

		{"another session's state", other, otherCookie, oauth.Link, "ada-session", "not tied to this session"},
		{"tampered nonce", tampered(link, 1), planted(tampered(link, 1)), oauth.Link, "ada-session", "not tied to this session"},
		{"tampered MAC", tampered(link, 2), planted(tampered(link, 2)), oauth.Link, "ada-session", "not tied to this session"},
		{"link without its MAC", unbound, planted(unbound), oauth.Link, "ada-session", "not tied to this session"},

		{"link state for sign-in", link, linkCookie, oauth.SignIn, "", "not for a signin flow"},
		{"sign-in state for link", signIn, signInCookie, oauth.Link, "ada-session", "not for a link flow"},
		{"sign-in state with a MAC", signIn + ".00", planted(signIn + ".00"), oauth.SignIn, "", "not for a signin flow"},
		{"tampered kind", wrongKind, planted(wrongKind), oauth.Link, "ada-session", "not for a link flow"},
		{"planted state", "X", planted("X"), oauth.Link, "ada-session", "not for a link flow"},
		{"planted sign-in state without a nonce", "signin.", planted("signin."), oauth.SignIn, "", "not for a signin flow"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := states.Verify(callback(c.state, c.cookie), c.kind, c.session)
			if c.want == "" && err != nil {
				t.Fatalf("Verify = %v, want nil", err)
			}
			if c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
				t.Fatalf("Verify = %v, want an error with %q", err, c.want)
			}
		})
	}
}

func TestKindOf(t *testing.T) {
	for state, want := range map[string]oauth.Kind{
		"link.NONCE.MAC": oauth.Link,
		"signin.NONCE":   oauth.SignIn,
		"link":           oauth.Link,
		"":               "",
		"X":              "",
		"LINK.NONCE.MAC": "",
		"admin.NONCE":    "",
		".link.NONCE":    "",
	} {
		if got := oauth.KindOf(state); got != want {
			t.Errorf("KindOf(%q) = %q, want %q", state, got, want)
		}
	}
}

func TestFlow(t *testing.T) {
	signIn, signInCookie := issue(t, states, oauth.SignIn, "")
	link, linkCookie := issue(t, states, oauth.Link, "session-token")
	for _, c := range []struct {
		name   string
		query  string // the callback's raw query
		cookie *http.Cookie
		want   oauth.Kind
	}{
		{"sign-in state", "state=" + url.QueryEscape(signIn), signInCookie, oauth.SignIn},
		{"link state", "state=" + url.QueryEscape(link), linkCookie, oauth.Link},
		// The query's state decides, whatever the cookie says; Verify then refuses the mismatch.
		{"sign-in state with a link cookie", "state=" + url.QueryEscape(signIn), linkCookie, oauth.SignIn},
		{"link state with a sign-in cookie", "state=" + url.QueryEscape(link), signInCookie, oauth.Link},
		{"planted state", "state=X", signInCookie, ""},
		{"no state, sign-in cookie", "error=access_denied", signInCookie, oauth.SignIn},
		{"empty state, sign-in cookie", "state=", signInCookie, oauth.SignIn},
		{"no state, link cookie", "code=c", linkCookie, oauth.Link},
		{"no state or cookie", "code=c", nil, ""},
		{"another cookie", "code=c", &http.Cookie{Name: "other_state", Value: signIn}, ""},
	} {
		r := httptest.NewRequest("GET", "/api/test/callback?"+c.query, nil)
		if c.cookie != nil {
			r.AddCookie(&http.Cookie{Name: c.cookie.Name, Value: c.cookie.Value})
		}
		if got := states.Flow(r); got != c.want {
			t.Errorf("%s: Flow = %q, want %q", c.name, got, c.want)
		}
	}
}
