package app_test

import (
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

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
