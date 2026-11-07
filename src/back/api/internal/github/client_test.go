package github_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gantt-stop-me/api/internal/github"
)

func TestRateLimitedForbiddenIsTooManyRequests(t *testing.T) {
	for _, c := range []struct {
		remaining, message string
		want               int
	}{
		{"0", "API rate limit exceeded for user ID 1.", http.StatusTooManyRequests},
		{"4999", "You have exceeded a secondary rate limit. Please wait a few minutes before you try again.", http.StatusTooManyRequests},
		{"4999", "Resource protected by organization SAML enforcement.", http.StatusForbidden},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", c.remaining)
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"` + c.message + `"}`))
		}))
		gh := &github.Client{API: srv.URL, HTTP: srv.Client()}
		_, err := gh.RepoByID(t.Context(), "token", 1)
		srv.Close()
		if ghErr := (*github.Error)(nil); !errors.As(err, &ghErr) || ghErr.Status != c.want {
			t.Errorf("403 %q with %s remaining: err = %v, want HTTP %d", c.message, c.remaining, err, c.want)
		}
	}
}
