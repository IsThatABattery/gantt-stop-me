package runner

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestMCP(t *testing.T) {
	s := &Server{}
	c := fakeExecution(t, s, map[string]string{})
	ctx := context.Background()
	_, _, _, err := c.MCP(ctx, 1, "", "", []byte(`{}`))
	if e := (*Error)(nil); !errors.As(err, &e) || e.Status != 409 || e.Code != "no_browser" {
		t.Errorf("MCP() before the browser = %v, want 409 no_browser", err)
	}

	// A browser on this machine's loopback, recording what reaches it.
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", browserPort))
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan http.Header, 1)
	browser := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		h := r.Header.Clone()
		h.Set("Host", r.Host)
		got <- h
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Mcp-Session-Id", "s1")
		w.Header().Set("X-Other", "dropped")
		if string(body) == "big" {
			io.WriteString(w, strings.Repeat("x", maxAnswer+1))
			return
		}
		io.WriteString(w, "data: "+string(body)+"\n\n")
	})}
	go browser.Serve(l)
	defer browser.Close()
	s.setAddr(s.execs[1], "browser", "127.0.0.1")

	answer, contentType, session, err := c.MCP(ctx, 1, "s1", "2025-06-18", []byte(`{"jsonrpc": "2.0"}`))
	if err != nil || string(answer) != "data: {\"jsonrpc\": \"2.0\"}\n\n" || contentType != "text/event-stream" || session != "s1" {
		t.Errorf("MCP() = %q %q %q, %v", answer, contentType, session, err)
	}
	h := <-got
	for k, want := range map[string]string{
		"Host": browserHost, "Content-Type": "application/json", "Accept": "application/json, text/event-stream",
		"Mcp-Session-Id": "s1", "Mcp-Protocol-Version": "2025-06-18",
		"Authorization": "", // the runner's token stays with the runner
	} {
		if h.Get(k) != want {
			t.Errorf("the browser got %s %q, want %q", k, h.Get(k), want)
		}
	}

	_, _, _, err = c.MCP(ctx, 1, "s1", "2025-06-18", []byte("big"))
	<-got
	if e := (*Error)(nil); !errors.As(err, &e) || e.Code != "too_large" {
		t.Errorf("MCP() of an answer over %d MiB = %v, want too_large", maxAnswer>>20, err)
	}
}
