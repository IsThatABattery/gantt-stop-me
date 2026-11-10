package oauth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"gantt-stop-me/api/internal/oauth"
)

// endpoint serves a token endpoint that hands each request's form to got and answers status with body.
func endpoint(t *testing.T, status int, body string, got func(r *http.Request, form url.Values)) *oauth.Endpoint {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if got != nil {
			got(r, r.PostForm)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &oauth.Endpoint{TokenURL: srv.URL + "/token", ClientID: "client-id", ClientSecret: "client-secret", HTTP: srv.Client()}
}

// expectForm fails the test unless r is a form POST to /token asking for JSON, with exactly want in its form.
func expectForm(t *testing.T, r *http.Request, form url.Values, want url.Values) {
	t.Helper()
	if r.Method != http.MethodPost || r.URL.Path != "/token" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
		r.Header.Get("Accept") != "application/json" {
		t.Errorf("%s %s with Content-Type %q, Accept %q; want a form POST to /token asking for JSON", r.Method, r.URL.Path,
			r.Header.Get("Content-Type"), r.Header.Get("Accept"))
	}
	if r.URL.RawQuery != "" {
		t.Errorf("query %q, want the credentials in the body alone", r.URL.RawQuery)
	}
	if a, b := form.Encode(), want.Encode(); a != b {
		t.Errorf("form %s, want %s", a, b)
	}
}

func TestExchange(t *testing.T) {
	e := endpoint(t, http.StatusOK, `{"access_token":"at","token_type":"bearer","scope":"repo","expires_in":28800,
		"refresh_token":"rt","refresh_token_expires_in":15897600}`, func(r *http.Request, form url.Values) {
		expectForm(t, r, form, url.Values{"grant_type": {"authorization_code"}, "code": {"the-code"},
			"redirect_uri": {"http://app.test/callback"}, "client_id": {"client-id"}, "client_secret": {"client-secret"}})
	})
	before := time.Now()
	tok, err := e.Exchange(t.Context(), "the-code", "http://app.test/callback")
	after := time.Now()
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "at" || tok.RefreshToken != "rt" {
		t.Fatalf("Exchange = %+v, want access token at and refresh token rt", tok)
	}
	if tok.Expires.Before(before.Add(8*time.Hour)) || tok.Expires.After(after.Add(8*time.Hour)) {
		t.Fatalf("Expires = %v, want 8 hours from the exchange", tok.Expires)
	}
	if want := 15897600 * time.Second; tok.RefreshExpires.Before(before.Add(want)) || tok.RefreshExpires.After(after.Add(want)) {
		t.Fatalf("RefreshExpires = %v, want 184 days from the exchange", tok.RefreshExpires)
	}
}

func TestExchangeTokenThatDoesNotExpire(t *testing.T) {
	tok, err := endpoint(t, http.StatusOK, `{"access_token":"at","token_type":"bearer"}`, nil).Exchange(t.Context(), "c", "u")
	if err != nil || tok != (oauth.Token{AccessToken: "at"}) {
		t.Fatalf("Exchange = %+v, %v; want access token at alone, with no expiry", tok, err)
	}
}

func TestRefresh(t *testing.T) {
	e := endpoint(t, http.StatusOK, `{"access_token":"at2","expires_in":60,"refresh_token":"rt2","refresh_token_expires_in":120}`,
		func(r *http.Request, form url.Values) {
			expectForm(t, r, form, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"rt"},
				"client_id": {"client-id"}, "client_secret": {"client-secret"}})
		})
	tok, err := e.Refresh(t.Context(), "rt")
	if err != nil || tok.AccessToken != "at2" || tok.RefreshToken != "rt2" ||
		time.Until(tok.Expires) > time.Minute || time.Until(tok.RefreshExpires) < time.Minute {
		t.Fatalf("Refresh = %+v, %v; want at2 for a minute and rt2 for two", tok, err)
	}
}

func TestTokenEndpointRefuses(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		body   string
		want   oauth.Error
	}{
		// GitHub answers a refusal with 200.
		{"200", http.StatusOK, `{"error":"bad_verification_code","error_description":"The code passed is incorrect or expired."}`,
			oauth.Error{Code: "bad_verification_code", Description: "The code passed is incorrect or expired."}},
		{"200 without a description", http.StatusOK, `{"error":"bad_refresh_token"}`, oauth.Error{Code: "bad_refresh_token"}},
		// RFC 6749 answers it with 400, or 401 for the client's credentials.
		{"400", http.StatusBadRequest, `{"error":"invalid_grant"}`, oauth.Error{Code: "invalid_grant"}},
		{"401", http.StatusUnauthorized, `{"error":"invalid_client","error_description":"Unknown client."}`,
			oauth.Error{Code: "invalid_client", Description: "Unknown client."}},
		// An error wins over a token sent with it.
		{"200 with a token", http.StatusOK, `{"access_token":"at","error":"incorrect_client_credentials"}`,
			oauth.Error{Code: "incorrect_client_credentials"}},
	} {
		for name, call := range map[string]func(*oauth.Endpoint) (oauth.Token, error){
			"Exchange": func(e *oauth.Endpoint) (oauth.Token, error) { return e.Exchange(t.Context(), "c", "u") },
			"Refresh":  func(e *oauth.Endpoint) (oauth.Token, error) { return e.Refresh(t.Context(), "rt") },
		} {
			tok, err := call(endpoint(t, c.status, c.body, nil))
			if oauthErr := (*oauth.Error)(nil); !errors.As(err, &oauthErr) || *oauthErr != c.want || tok != (oauth.Token{}) {
				t.Errorf("%s, %s: %s = %+v, %v; want %+v", c.name, c.body, name, tok, err, c.want)
			}
		}
	}
}

func TestErrorQuotes(t *testing.T) {
	err := &oauth.Error{Code: "e\n2026/11/09 00:00:00 forged", Description: "d\nforged too"}
	if got, want := err.Error(), `oauth: the token endpoint refused: "e\n2026/11/09 00:00:00 forged": "d\nforged too"`; got != want {
		t.Fatalf("Error() = %s, want %s", got, want)
	}
	if got := (&oauth.Error{Code: "bad_refresh_token"}).Error(); got != `oauth: the token endpoint refused: "bad_refresh_token"` {
		t.Fatalf("Error() = %s", got)
	}
}

func TestTokenEndpointFails(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		body   string
	}{
		// GitHub answers a client id it does not know with 404.
		{"404", http.StatusNotFound, `{"error":"Not Found"}`},
		{"404 without JSON", http.StatusNotFound, `Not Found`},
		{"403", http.StatusForbidden, `{"message":"Forbidden"}`},
		{"429", http.StatusTooManyRequests, `{"error":"slow_down"}`},
		{"500", http.StatusInternalServerError, `{"error":"server_error"}`},
		{"503 without a body", http.StatusServiceUnavailable, ``},
	} {
		tok, err := endpoint(t, c.status, c.body, nil).Exchange(t.Context(), "c", "u")
		statusErr := (*oauth.StatusError)(nil)
		if !errors.As(err, &statusErr) || statusErr.Status != c.status || tok != (oauth.Token{}) {
			t.Errorf("%s: Exchange = %+v, %v; want a StatusError for HTTP %d", c.name, tok, err, c.status)
		}
		if oauthErr := (*oauth.Error)(nil); errors.As(err, &oauthErr) {
			t.Errorf("%s: Exchange = %v, a refusal, which asking again gets", c.name, err)
		}
	}
}

func TestTokenEndpointAnswersNoToken(t *testing.T) {
	for _, body := range []string{`{}`, `{"access_token":""}`, `access_token=at&token_type=bearer`, ``} {
		tok, err := endpoint(t, http.StatusOK, body, nil).Exchange(t.Context(), "c", "u")
		if err == nil || tok != (oauth.Token{}) {
			t.Errorf("%q: Exchange = %+v, %v; want an error", body, tok, err)
		}
		statusErr, oauthErr := (*oauth.StatusError)(nil), (*oauth.Error)(nil)
		if errors.As(err, &statusErr) || errors.As(err, &oauthErr) {
			t.Errorf("%q: Exchange = %v, want neither a refusal nor a status", body, err)
		}
	}
}

func TestTokenEndpointLogsNoSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler) // drops the connection
	}))
	defer srv.Close()
	e := &oauth.Endpoint{TokenURL: srv.URL + "/token", ClientID: "client-id", ClientSecret: "client-secret", HTTP: srv.Client()}
	_, err := e.Refresh(t.Context(), "the-refresh-token")
	if err == nil {
		t.Fatal("Refresh over a dropped connection: no error")
	}
	for _, secret := range []string{"client-secret", "the-refresh-token"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("the error has %q: %v", secret, err)
		}
	}
}
