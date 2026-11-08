package runner

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeExecution serves s with execution 1 already prepared, its containers at addrs, and returns a client for it.
func fakeExecution(t *testing.T, s *Server, addrs map[string]string) *Client {
	t.Helper()
	port := 80
	s.Token = "t"
	s.execs = map[int64]*execution{1: {id: 1, spec: Spec{Port: &port}, ctx: context.Background(), addrs: addrs}}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return &Client{URL: srv.URL, Token: "t", HTTPClient: srv.Client()}
}

func TestRequest(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/moved":
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		case "/big":
			io.WriteString(w, strings.Repeat("x", 100<<10))
		default:
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("X-Host", r.Host)
			w.Header().Set("X-Got", r.Header.Get("X-Test"))
			w.WriteHeader(http.StatusTeapot)
			io.WriteString(w, r.Method+" "+string(body))
		}
	}))
	defer app.Close()
	_, port, _ := net.SplitHostPort(app.Listener.Addr().String())
	// The browser's address would serve too, but /http must never reach it.
	c := fakeExecution(t, &Server{}, map[string]string{"app": "127.0.0.1", "db": "127.0.0.1", "browser": "127.0.0.1"})
	ctx := context.Background()

	res, err := c.HTTP(ctx, 1, HTTPRequest{Method: "POST", URL: "http://app:" + port + "/signup", Headers: map[string]string{"X-Test": "1"}, Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != http.StatusTeapot || res.Body != "POST hi" || res.Headers.Get("X-Got") != "1" {
		t.Errorf("POST app = %d %q %v", res.Status, res.Body, res.Headers)
	}
	if host := res.Headers.Get("X-Host"); host != "app:"+port {
		t.Errorf("Host header = %q, want the URL's app:%s", host, port)
	}
	if res, err := c.HTTP(ctx, 1, HTTPRequest{URL: "http://db:" + port + "/"}); err != nil || res.Body != "GET " {
		t.Errorf("GET db = %+v, %v", res, err)
	}
	if res, err := c.HTTP(ctx, 1, HTTPRequest{URL: "http://app:" + port + "/moved"}); err != nil || res.Status != http.StatusFound {
		t.Errorf("a redirect = %+v, %v; want the 302 itself", res, err)
	}
	if res, err := c.HTTP(ctx, 1, HTTPRequest{URL: "http://app:" + port + "/big"}); err != nil || len(res.Body) != 64<<10 {
		t.Errorf("a 100 KB body came back as %d bytes, %v; want 64 KB", len(res.Body), err)
	}

	for _, c2 := range []struct{ url, code string }{
		{"https://app:" + port + "/", "bad_url"},
		{"ftp://app:" + port + "/", "bad_url"},
		{"app:" + port, "bad_url"},
		{"//app:" + port + "/", "bad_url"},
		{"http://browser:" + port + "/", "bad_host"},
		{"http://127.0.0.1:" + port + "/", "bad_host"},
		{"http://localhost:" + port + "/", "bad_host"},
		{"http://example.com/", "bad_host"},
		{"http://APP:" + port + "/", "bad_host"},
		{"http://other:" + port + "/", "bad_host"},
	} {
		_, err := c.HTTP(ctx, 1, HTTPRequest{URL: c2.url})
		if e := (*Error)(nil); !errors.As(err, &e) || e.Status != 400 || e.Code != c2.code {
			t.Errorf("%s: %v, want 400 %s", c2.url, err, c2.code)
		}
	}

	app.Close()
	_, err = c.HTTP(ctx, 1, HTTPRequest{URL: "http://app:" + port + "/"})
	if e := (*Error)(nil); !errors.As(err, &e) || e.Status != http.StatusBadGateway || e.Code != "unreachable" {
		t.Errorf("a closed app: %v, want 502 unreachable", err)
	}
}
