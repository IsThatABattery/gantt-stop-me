package runner

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestDocker drives one execution against real Docker, from the runner-test container:
// docker compose --profile test run --rm runner-test
func TestDocker(t *testing.T) {
	if os.Getenv("RUNNER_DOCKER_TESTS") != "1" {
		t.Skip("needs Docker; run it with docker compose --profile test run --rm runner-test")
	}
	ctx := context.Background()
	self, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Token: "t", Prefix: cmp.Or(os.Getenv("RUN_PREFIX"), "gantt-test"), Work: t.TempDir(), Self: self}
	if err := s.DeleteAll(ctx); err != nil { // what a failed earlier run left
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DeleteAll(context.WithoutCancel(ctx)) })
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	c := &Client{URL: srv.URL, Token: "t", HTTPClient: srv.Client()}

	port := 80
	spec := Spec{
		Project: 1, Workflow: "smoke", Context: ".", Dockerfile: "Dockerfile",
		Env:   map[string]string{"GREETING": "hi from env"},
		Cache: []string{"/cache"},
		Port:  &port,
		Services: []Service{{Name: "db", Image: "postgres:18", Env: map[string]string{"POSTGRES_PASSWORD": "test"},
			Health: "pg_isready -h localhost -U postgres"}},
	}
	source := tarball(t,
		dir("owner-repo-abc123/"),
		file("owner-repo-abc123/Dockerfile", "FROM busybox:1.37\nCOPY www /www\nCMD [\"httpd\", \"-f\", \"-p\", \"80\", \"-h\", \"/www\"]\n"),
		file("owner-repo-abc123/www/index.html", "hello from app\n"),
	)
	if _, err := c.Prepare(ctx, 1, spec, bytes.NewReader(source)); err != nil {
		if f := (*Failure)(nil); errors.As(err, &f) {
			t.Fatalf("Prepare: %v\n%s", err, f.Log)
		}
		t.Fatalf("Prepare: %v", err)
	}

	code, out, err := c.Command(ctx, 1, `wget -q -O- http://app/ && echo "$GREETING" && touch /cache/ok`)
	if err != nil || code != 0 || !strings.Contains(out, "hello from app") || !strings.Contains(out, "hi from env") {
		t.Errorf("a command calling the app = %d %q, %v", code, out, err)
	}
	if code, out, err := c.Command(ctx, 1, "ls /cache; exit 3"); err != nil || code != 3 || !strings.Contains(out, "ok") {
		t.Errorf("a failing command = %d %q, %v; want exit 3 and the cache kept", code, out, err)
	}
	if code, out, err := c.Command(ctx, 1, "wget -T 3 -q -O- http://example.com/"); err != nil || code == 0 {
		t.Errorf("a command reached the internet: %d %q, %v", code, out, err)
	}

	res, err := c.HTTP(ctx, 1, HTTPRequest{URL: "http://app/"})
	if err != nil || res.Status != 200 || res.Body != "hello from app\n" {
		t.Errorf("GET http://app/ = %+v, %v", res, err)
	}

	for range 2 { // safe to repeat
		if err := c.Browser(ctx, 1); err != nil {
			if f := (*Failure)(nil); errors.As(err, &f) {
				t.Fatalf("Browser: %v\n%s", err, f.Log)
			}
			t.Fatalf("Browser: %v", err)
		}
	}
	_, _, session, err := c.MCP(ctx, 1, "", []byte(`{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params":
		{"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "runner-test", "version": "1"}}}`))
	if err != nil || session == "" {
		t.Fatalf("initialize = session %q, %v", session, err)
	}
	if _, _, _, err := c.MCP(ctx, 1, session, []byte(`{"jsonrpc": "2.0", "method": "notifications/initialized"}`)); err != nil {
		t.Fatalf("notifications/initialized: %v", err)
	}
	answer, contentType, _, err := c.MCP(ctx, 1, session, []byte(`{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}`))
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if strings.HasPrefix(contentType, "text/event-stream") { // one event, its JSON on the data line
		for line := range strings.Lines(string(answer)) {
			if data, ok := strings.CutPrefix(line, "data: "); ok {
				answer = []byte(data)
			}
		}
	}
	var list struct {
		Result struct {
			Tools []struct{ Name string } `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(answer, &list); err != nil {
		t.Fatalf("tools/list answer %q: %v", answer, err)
	}
	var names []string
	for _, tool := range list.Result.Tools {
		names = append(names, tool.Name)
	}
	if !strings.Contains(" "+strings.Join(names, " ")+" ", " browser_navigate ") {
		t.Errorf("tools/list = %v, want browser_navigate among them", names)
	}

	// DELETE stops a command still running.
	ran := make(chan error, 1)
	go func() {
		_, _, err := c.Command(ctx, 1, "sleep 300")
		ran <- err
	}()
	for i := 0; i < 100; i++ {
		if running, _ := dockerOutput(ctx, "ps", "--quiet", "--filter", "name="+s.network(1)+"_"); running != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	for range 2 { // safe to repeat
		if err := c.Delete(ctx, 1); err != nil {
			t.Fatalf("Delete: %v", err)
		}
	}
	if err := <-ran; err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("a command running through DELETE = %v, want 409 cancelled", err)
	}
	for _, list := range [][]string{{"ps", "--all"}, {"network", "ls"}} {
		left, err := dockerOutput(ctx, append(list, "--quiet", "--filter", "label=gantt.prefix="+s.Prefix)...)
		if err != nil || left != "" {
			t.Errorf("docker %s left %q, %v", list[0], left, err)
		}
	}
}
