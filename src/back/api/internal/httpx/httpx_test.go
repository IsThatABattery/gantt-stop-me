package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckOrigin(t *testing.T) {
	h := CheckOrigin("http://app.test/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	cases := []struct {
		method, origin string
		want           int
	}{
		{"GET", "", http.StatusTeapot},
		{"GET", "http://evil.test", http.StatusTeapot},
		{"POST", "http://app.test", http.StatusTeapot},
		{"PATCH", "http://app.test", http.StatusTeapot},
		{"POST", "", http.StatusForbidden},
		{"POST", "http://evil.test", http.StatusForbidden},
		{"POST", "https://app.test", http.StatusForbidden},
		{"DELETE", "http://app.test.evil.test", http.StatusForbidden},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, "/api/x", nil)
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s with Origin %q: status %d, want %d", c.method, c.origin, rec.Code, c.want)
		}
	}
}
