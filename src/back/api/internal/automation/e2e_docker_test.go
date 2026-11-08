package automation_test

import (
	"bytes"
	"cmp"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/runner"
)

// TestRunOnDocker carries one test run from a ready pull request to its merge across every slice, with the real
// runner on real Docker in place of the fake, from the runner-test container:
// docker compose --profile test run --rm runner-test
func TestRunOnDocker(t *testing.T) {
	if os.Getenv("RUNNER_DOCKER_TESTS") != "1" {
		t.Skip("needs Docker; run it with docker compose --profile test run --rm runner-test")
	}
	ctx := context.Background()
	self, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	s := &runner.Server{Token: "e2e", Prefix: cmp.Or(os.Getenv("RUN_PREFIX"), "gantt-test"), Work: t.TempDir(), Self: self}
	if err := s.DeleteAll(ctx); err != nil { // what a failed earlier run left
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DeleteAll(ctx) })
	// On loopback, which no test container can reach.
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	f := withLLM(t, func(c *app.Config) {
		c.RunnerURL = srv.URL
		c.Runner = &runner.Client{URL: srv.URL, Token: s.Token, HTTPClient: srv.Client()}
	})

	const page = "hello from octo/hello"
	f.repo.AddFile("a1", "Dockerfile", "FROM busybox:1.37\nCOPY www /www\nCMD [\"httpd\", \"-f\", \"-p\", \"80\", \"-h\", \"/www\"]\n")
	f.repo.AddFile("a1", "www/index.html", page+"\n")
	f.repo.AddFile("a1", ".gantt/workflows.yml", `workflows:
  web:
    description: The app and its database
    port: 80
    services:
      db:
        image: postgres:18
        env:
          POSTGRES_PASSWORD: test
        health: pg_isready -h localhost -U postgres
`)
	f.tests(1, "## Tests\n- [ ] `wget -qO- http://app/ | grep '"+page+"'`\n- [ ] The home page greets the visitor\n")
	f.llm.Reply(
		call("http_request", `{"method": "GET", "url": "http://app/"}`),
		call("browser_navigate", `{"url": "http://app/"}`),
		call("browser_snapshot", `{}`), // navigate links its snapshot as a file, which the model cannot open
		reported("pass", "The page says hello."),
	)
	n := f.ready()
	f.work() // drives the execution to its end, then finishes the run and posts its comment

	var log string
	f.DB.QueryRow(ctx, `select log_tail from workflow_executions`).Scan(&log)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("execution log:\n%s", log)
		}
	})
	f.expectRuns(pass(n))
	got := f.checks()
	if len(got) != 2 || got[0].Status != "passed" || !strings.Contains(got[0].Detail, page) ||
		got[1].Status != "passed" || got[1].Detail != "The page says hello." {
		t.Fatalf("results = %+v", got)
	}
	if steps := got[1].Transcript; len(steps) != 4 || !strings.Contains(steps[0].Answer, `"status":200`) ||
		!strings.Contains(steps[0].Answer, page) || !strings.Contains(steps[2].Answer, page) {
		t.Fatalf("transcript = %+v, want http_request and the browser to show the app's page", steps)
	}
	if png := got[1].Screenshot; !bytes.HasPrefix(png, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("screenshot = %d bytes, want a PNG", len(png))
	}
	if got := f.task(1); got.Column != "user-check" {
		t.Fatalf("task = %+v, want it in User Check", got)
	}
	if c := f.repo.Issue(1).Comments; len(c) != 1 || !strings.HasPrefix(c[0].Body, "**Tests passed** on a1.") {
		t.Fatalf("comments = %+v, want one saying the tests passed", c)
	}

	f.approve(f.ada).Expect(http.StatusOK, "")
	if pr := f.repo.Issue(n); pr.State != "closed" || pr.Pull.MergedWith == "" {
		t.Fatalf("pull request = %s, merged with %q; want it merged", pr.State, pr.Pull.MergedWith)
	}
	if got := f.task(1); got.Column != "closed" {
		t.Fatalf("task = %+v, want it in Closed", got)
	}

	for _, list := range [][]string{{"ps", "--all"}, {"network", "ls"}} {
		out, err := exec.Command("docker", append(list, "--quiet", "--filter", "label=gantt.prefix="+s.Prefix)...).CombinedOutput()
		if err != nil || len(out) != 0 {
			t.Errorf("docker %s left %q, %v", list[0], out, err)
		}
	}
}
