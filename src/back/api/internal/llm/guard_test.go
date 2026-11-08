package llm

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDialGuard(t *testing.T) {
	cases := []struct {
		address        string
		public, anyway bool // allowed without allowPrivate, and with it
	}{
		{"8.8.8.8:443", true, true},
		{"[2001:4860:4860::8888]:443", true, true},
		{"127.0.0.1:80", false, true},
		{"[::1]:80", false, true},
		{"10.0.0.1:80", false, true},
		{"172.16.0.1:80", false, true},
		{"192.168.1.1:80", false, true},
		{"100.64.0.1:80", false, true},
		{"[fd00::1]:80", false, true},
		{"[::ffff:10.0.0.1]:80", false, true},
		{"169.254.169.254:80", false, false},
		{"[::ffff:169.254.169.254]:80", false, false},
		{"[fe80::1]:80", false, false},
		{"224.0.0.1:80", false, false},
		{"0.0.0.0:80", false, false},
		{"[::]:80", false, false},
	}
	for _, c := range cases {
		if err := dialGuard(false)("tcp", c.address, nil); (err == nil) != c.public {
			t.Errorf("%s without allowPrivate: %v", c.address, err)
		}
		if err := dialGuard(true)("tcp", c.address, nil); (err == nil) != c.anyway {
			t.Errorf("%s with allowPrivate: %v", c.address, err)
		}
	}
	if dialGuard(true)("tcp", "not an address", nil) == nil {
		t.Error("an unparsable address was allowed")
	}
}

func TestHTTPClient(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/slow":
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		}
	}))
	t.Cleanup(srv.Close)

	if _, err := HTTPClient(false, CallTimeout).Get(srv.URL + "/ok"); !errors.Is(err, errBlocked) || hits.Load() != 0 {
		t.Fatalf("loopback without allowPrivate: %v, %d requests served", err, hits.Load())
	}
	res, err := HTTPClient(true, CallTimeout).Get(srv.URL + "/redirect")
	if err != nil || res.StatusCode != http.StatusFound || hits.Load() != 1 {
		t.Fatalf("redirect: %v, %v, %d requests served; want the 302 itself", res, err, hits.Load())
	}
	res.Body.Close()
	start := time.Now()
	if _, err := HTTPClient(true, 50*time.Millisecond).Get(srv.URL + "/slow"); err == nil || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("slow answer: %v after %v, want a timeout", err, time.Since(start))
	}
}
