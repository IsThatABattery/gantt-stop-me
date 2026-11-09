package automation_test

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/tasktests"
)

// TestLiveLLMOnDocker is TestRunOnDocker with a real provider in place of the fake: the decision model routes two
// unpinned checks between two workflows, and the chat model verifies them in the browser, one the page bears out and
// one it contradicts, so a model that passes everything fails it. It spends real calls, so it runs only with a key,
// from the runner-test container: LLM_LIVE_KEY, LLM_LIVE_DECISION_MODEL, LLM_LIVE_CHAT_MODEL and, for a provider
// other than OpenAI, LLM_LIVE_BASE_URL. With LLM_LIVE_ARTIFACTS, it saves each check's screenshot and video there.
func TestLiveLLMOnDocker(t *testing.T) {
	key, decision, chat := os.Getenv("LLM_LIVE_KEY"), os.Getenv("LLM_LIVE_DECISION_MODEL"), os.Getenv("LLM_LIVE_CHAT_MODEL")
	if key == "" {
		t.Skip("calls a real provider; set LLM_LIVE_KEY and run it in runner-test")
	}
	if decision == "" || chat == "" {
		t.Fatal("set LLM_LIVE_DECISION_MODEL and LLM_LIVE_CHAT_MODEL")
	}
	_, docker := onDocker(t)
	f := setup(t, docker)
	f.automate(cmp.Or(os.Getenv("LLM_LIVE_BASE_URL"), "https://api.openai.com/v1"), key, decision, chat)

	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Title = "Show today's special on the home page" })
	f.repo.AddFile("a1", "Dockerfile", "FROM busybox:1.37\nCOPY www /www\nCMD [\"httpd\", \"-f\", \"-p\", \"80\", \"-h\", \"/www\"]\n")
	f.repo.AddFile("a1", "www/index.html", "<h1>Octo Bakery</h1>\n<p>Today's special: cardamom buns, 4 euros each.</p>\n")
	f.repo.AddFile("a1", ".gantt/workflows.yml", `workflows:
  web:
    description: The bakery's website, served on port 80
    port: 80
  worker:
    description: The nightly invoicing batch job, which serves no pages
`)
	f.setTests(1,
		tasktests.Test{Name: "The home page shows today's special for 4 euros", Kind: "check"},
		tasktests.Test{Name: "The home page shows today's special: cinnamon rolls for 3 euros", Kind: "check"},
	)
	n := f.ready()
	f.work()

	got, routes := f.checks(), f.results()
	for i, p := range f.probabilities() {
		t.Logf("test %d routed to %q, probabilities %v", i+1, routes[i].Workflow, p)
		if routes[i].Workflow != "web" || p["web"] < 0.5 {
			t.Errorf("test %d routed to %q with web at %v, want web at 0.5 or more", i+1, routes[i].Workflow, p["web"])
		}
	}
	dir := os.Getenv("LLM_LIVE_ARTIFACTS")
	for i, c := range got {
		var tools []string
		for _, s := range c.Transcript {
			tools = append(tools, s.Tool)
		}
		t.Logf("test %d %s after %v: %s", i+1, c.Status, tools, c.Detail)
		// Only a browser tool takes a screenshot, so http_request alone fails this.
		if !bytes.HasPrefix(c.Screenshot, []byte("\x89PNG\r\n\x1a\n")) {
			t.Errorf("test %d screenshot = %d bytes, want a PNG from the browser", i+1, len(c.Screenshot))
		}
		if dir != "" {
			save(t, filepath.Join(dir, fmt.Sprintf("check-%d.png", i+1)), c.Screenshot)
			save(t, filepath.Join(dir, fmt.Sprintf("check-%d.webm", i+1)), c.Video)
		}
	}
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "failed", Finished: true}) // says why, when planning failed
	if len(got) != 2 || got[0].Status != "passed" || got[1].Status != "failed" {
		t.Fatal("want the first check passed and the second failed")
	}
	// Only the page names the special, so the failing check's reason can quote it only from the page.
	if !strings.Contains(strings.ToLower(got[1].Detail), "cardamom") {
		t.Errorf("test 2 detail %q, want it to say what the page shows", got[1].Detail)
	}
}

// save writes data to path, when there is any.
func save(t *testing.T, path string, data []byte) {
	t.Helper()
	if len(data) == 0 {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
