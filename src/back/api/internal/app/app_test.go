package app_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestOriginCheckSkipsOnlyTheWebhook(t *testing.T) {
	env := apptest.New(t)
	// send makes a request with no Origin, as GitHub's deliveries come, and returns its status and error code.
	send := func(method, path string) (int, string) {
		req, err := http.NewRequest(method, env.Server.URL+path, strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var body struct {
			Error struct{ Code string }
		}
		json.NewDecoder(res.Body).Decode(&body)
		return res.StatusCode, body.Error.Code
	}
	// The webhook checks its signature instead, and project 1 has no secret.
	if status, code := send("POST", "/api/github/webhook/1"); status != http.StatusUnauthorized || code != "invalid_signature" {
		t.Errorf("POST /api/github/webhook/1 = %d %s, want 401 invalid_signature", status, code)
	}
	for _, c := range []struct{ method, path string }{
		{"PUT", "/api/github/webhook/1"},
		{"DELETE", "/api/github/webhook/1"},
		{"POST", "/api/github/webhook"},
		{"POST", "/api/github/webhooks/1"},
		{"POST", "/api/github/token"},
	} {
		if status, code := send(c.method, c.path); status != http.StatusForbidden || code != "bad_origin" {
			t.Errorf("%s %s = %d %s, want 403 bad_origin", c.method, c.path, status, code)
		}
	}
}

func TestUnknownRoutesUseErrorShape(t *testing.T) {
	env := apptest.New(t)
	res := env.Do("GET", "/api/auth/login", nil)
	res.Expect(http.StatusMethodNotAllowed, "method_not_allowed")
	if got := res.Header.Get("Allow"); got != "POST" {
		t.Errorf("Allow = %q, want POST", got)
	}
	env.Do("DELETE", "/api/me", nil).Expect(http.StatusMethodNotAllowed, "method_not_allowed")
	env.Do("GET", "/api/nope", nil).Expect(http.StatusNotFound, "not_found")
	// An unclean path still redirects to the clean one, which then answers.
	for _, method := range []string{"GET", "DELETE"} {
		if res := env.Do(method, "/api//me", nil); res.Status/100 != 3 || res.Header.Get("Location") != "/api/me" {
			t.Errorf("%s /api//me = %d to %q, want a redirect to /api/me", method, res.Status, res.Header.Get("Location"))
		}
	}
}
