package runner

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuth(t *testing.T) {
	srv := httptest.NewServer((&Server{Token: "secret", Prefix: "gantt"}).Handler())
	defer srv.Close()
	cases := []struct {
		method, path, auth string
		want               int
	}{
		{"GET", "/health", "", 200},
		{"POST", "/executions/1/commands", "", 401},
		{"POST", "/executions/1/commands", "Bearer wrong", 401},
		{"POST", "/executions/1/commands", "secret", 401},
		{"POST", "/executions/1/commands", "Bearer secret!", 401},
		{"POST", "/executions/1", "", 401},
		{"DELETE", "/executions", "", 401},
		{"POST", "/executions/1/mcp", "Bearer wrong", 401},
		{"POST", "/executions/1/commands", "Bearer secret", 404}, // let in: no such execution
	}
	for _, c := range cases {
		req, _ := http.NewRequest(c.method, srv.URL+c.path, nil)
		if c.auth != "" {
			req.Header.Set("Authorization", c.auth)
		}
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != c.want {
			t.Errorf("%s %s with %q = %d %s, want %d", c.method, c.path, c.auth, res.StatusCode, body, c.want)
		}
		if c.path == "/health" && string(body) != "ok" {
			t.Errorf("GET /health = %q, want ok", body)
		}
	}
}

func TestAuthWithoutToken(t *testing.T) {
	srv := httptest.NewServer((&Server{}).Handler())
	defer srv.Close()
	req, _ := http.NewRequest("POST", srv.URL+"/executions/1/commands", nil)
	req.Header.Set("Authorization", "Bearer ")
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Errorf("an empty token let a request in: %d", res.StatusCode)
	}
}
