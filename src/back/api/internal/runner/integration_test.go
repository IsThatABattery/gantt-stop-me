package runner

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	// Served the way main serves it, on the addresses it has before it joins an execution network.
	ls, err := Listen("8081")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: s.Handler()}
	for _, l := range ls {
		go srv.Serve(l)
	}
	defer srv.Close()
	c := &Client{URL: "http://127.0.0.1:8081", Token: "t", HTTPClient: http.DefaultClient}

	port, dbPort := 80, 5432
	spec := Spec{
		Project: 1, Workflow: "smoke", Context: ".", Dockerfile: "Dockerfile",
		Env:   map[string]string{"GREETING": "hi from env"},
		Cache: []string{"/cache"},
		Port:  &port,
		Services: []Service{{Name: "db", Image: "postgres:18", Env: map[string]string{"POSTGRES_PASSWORD": "test"},
			Health: "pg_isready -h localhost -U postgres", Port: &dbPort}},
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
	if code, out, err := c.Command(ctx, 1, "cp /bin/busybox /tmp/true && /tmp/true"); err != nil || code != 0 {
		t.Errorf("a binary in /tmp did not run: %d %q, %v", code, out, err)
	}
	if self, err := s.address(ctx, 1, s.Self); err != nil {
		t.Error(err)
	} else if code, out, err := c.Command(ctx, 1, "wget -T 3 -q -O- http://"+self+":8081/health"); err != nil || code == 0 {
		t.Errorf("a command reached the runner's API: %d %q, %v", code, out, err)
	}

	res, err := c.HTTP(ctx, 1, HTTPRequest{URL: "http://app/"})
	if err != nil || res.Status != 200 || res.Body != "hello from app\n" {
		t.Errorf("GET http://app/ = %+v, %v", res, err)
	}

	// A container a launch cut short left under the browser's name must not stop it starting.
	if _, err := dockerOutput(ctx, "create", "--name="+s.container(1, "browser"),
		"--label=gantt.prefix="+s.Prefix, "--label=gantt.execution=1", "busybox:1.37"); err != nil {
		t.Fatal(err)
	}
	for range 2 { // safe to repeat
		if err := c.Browser(ctx, 1); err != nil {
			if f := (*Failure)(nil); errors.As(err, &f) {
				t.Fatalf("Browser: %v\n%s", err, f.Log)
			}
			t.Fatalf("Browser: %v", err)
		}
	}
	const version = "2025-06-18"
	_, _, session, err := c.MCP(ctx, 1, "", "", []byte(`{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params":
		{"protocolVersion": "`+version+`", "capabilities": {}, "clientInfo": {"name": "runner-test", "version": "1"}}}`))
	if err != nil || session == "" {
		t.Fatalf("initialize = session %q, %v", session, err)
	}
	if _, _, _, err := c.MCP(ctx, 1, session, version, []byte(`{"jsonrpc": "2.0", "method": "notifications/initialized"}`)); err != nil {
		t.Fatalf("notifications/initialized: %v", err)
	}
	answer, contentType, _, err := c.MCP(ctx, 1, session, version, []byte(`{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}`))
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

	// A recording, on the browser's tmpfs, comes out once, then it is gone.
	for i, params := range []string{
		`{"name": "browser_start_video", "arguments": {"filename": "` + VideoFile("t1") + `"}}`,
		`{"name": "browser_navigate", "arguments": {"url": "http://app/"}}`,
		`{"name": "browser_stop_video", "arguments": {}}`,
	} {
		msg := fmt.Sprintf(`{"jsonrpc": "2.0", "id": %d, "method": "tools/call", "params": %s}`, 3+i, params)
		if answer, _, _, err := c.MCP(ctx, 1, session, version, []byte(msg)); err != nil || strings.Contains(string(answer), `"isError":true`) {
			t.Fatalf("%s = %s, %v", params, answer, err)
		}
	}
	if webm, err := c.Video(ctx, 1, "t1"); err != nil || !bytes.HasPrefix(webm, []byte("\x1a\x45\xdf\xa3")) {
		t.Errorf("Video() = %d bytes, %v; want a WebM", len(webm), err)
	}
	if _, err := c.Video(ctx, 1, "t1"); !isRunnerError(err, 404, "not_found") {
		t.Errorf("Video() again = %v, want 404 not_found", err)
	}
	// One over 64 MiB is refused, and deleted all the same.
	if _, err := dockerOutput(ctx, "exec", s.container(1, "browser"), "sh", "-c",
		fmt.Sprintf("head -c %d /dev/zero > %s", maxVideo+1, VideoFile("big"))); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Video(ctx, 1, "big"); !isRunnerError(err, 413, "too_large") {
		t.Errorf("Video() of a recording over 64 MiB = %v, want 413 too_large", err)
	}
	if _, err := c.Video(ctx, 1, "big"); !isRunnerError(err, 404, "not_found") {
		t.Errorf("Video() of it again = %v, want 404 not_found", err)
	}

	// Another project cannot build on this one's test image, which stays on the daemon.
	other := Spec{Project: 2, Workflow: "smoke", Context: ".", Dockerfile: "Dockerfile"}
	_, err = c.Prepare(ctx, 2, other, bytes.NewReader(tarball(t, file("r/Dockerfile", "FROM "+s.testImage(spec)+"\n"))))
	if f := (*Failure)(nil); !errors.As(err, &f) || f.Stage != "build" {
		t.Errorf("a build FROM another project's image = %v, want a build failure", err)
	}
	// An app that exits at once fails the start without waiting out the limit, its own output in the log.
	crash := Spec{Project: 1, Workflow: "crash", Context: ".", Dockerfile: "Dockerfile", Port: &port}
	start := time.Now()
	_, err = c.Prepare(ctx, 3, crash, bytes.NewReader(tarball(t, file("r/Dockerfile", "FROM busybox:1.37\nCMD echo \"app $((6*7)) died\"; exit 1\n"))))
	if f := (*Failure)(nil); !errors.As(err, &f) || f.Stage != "start" || !strings.Contains(f.Log, "app 42 died") {
		t.Errorf("an app that exits at once = %v, want a start failure with its output", err)
	} else if took := time.Since(start); took > limit/2 {
		t.Errorf("an app that exits at once took %s to fail", took)
	}
	for _, id := range []int64{2, 3} {
		if err := c.Delete(ctx, id); err != nil {
			t.Errorf("Delete(%d): %v", id, err)
		}
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
	deleted := make(chan error, 2)
	for range 2 { // safe to repeat, even at once
		go func() { deleted <- c.Delete(ctx, 1) }()
	}
	for range 2 {
		if err := <-deleted; err != nil {
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
