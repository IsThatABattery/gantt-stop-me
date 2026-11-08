package automation_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestWebhookChecksTheSignature(t *testing.T) {
	f := on(t)
	n := f.pull("ada", false, 1)
	body := fmt.Sprintf(`{"action": "opened", "number": %d}`, n)
	before := len(f.GitHub.Requests())
	refused := func(name, signature string) {
		t.Helper()
		if status, code := f.deliver(signature, "pull_request", body); status != http.StatusUnauthorized || code != "invalid_signature" {
			t.Errorf("%s: got %d %s, want 401 invalid_signature", name, status, code)
		}
	}

	refused("no secret generated yet", sign("", body))
	f.secret = f.webhookSecret(f.ada)
	refused("no signature", "")
	refused("another secret", sign("not the secret", body))
	refused("another body", sign(f.secret, body+" "))
	refused("no sha256= prefix", strings.TrimPrefix(sign(f.secret, body), "sha256="))
	refused("not hex", "sha256=zz")
	if got := f.GitHub.Requests()[before:]; len(got) != 0 {
		t.Fatalf("unsigned deliveries called GitHub: %v", got)
	}
	if r := f.record(n); r != nil {
		t.Fatalf("an unsigned delivery saved record %+v", r)
	}

	if status, _ := f.deliver(sign(f.secret, body), "pull_request", body); status != http.StatusAccepted {
		t.Fatalf("signed delivery answered %d, want 202", status)
	}
	f.expectRuns(testRun{Issue: 1, PR: n, Head: "a1", Status: "queued"})

	f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "")
	refused("automation off", sign(f.secret, body))
}

func TestWebhookIgnoresOtherEvents(t *testing.T) {
	f := automated(t)
	n := f.pull("ada", false, 1)
	before := len(f.GitHub.Requests())
	for _, event := range []string{"ping", "push", "issues"} {
		body := fmt.Sprintf(`{"zen": "Keep it logically awesome.", "number": %d}`, n)
		if status, _ := f.deliver(sign(f.secret, body), event, body); status != http.StatusAccepted {
			t.Errorf("%s answered %d, want 202", event, status)
		}
	}
	if got := f.GitHub.Requests()[before:]; len(got) != 0 {
		t.Fatalf("other events called GitHub: %v", got)
	}
	if r := f.record(n); r != nil {
		t.Fatalf("another event saved record %+v", r)
	}
}

func TestWebhookRefusesBadBodies(t *testing.T) {
	f := automated(t)
	n := f.pull("ada", false, 1)
	tooBig := fmt.Sprintf(`{"number": %d}`, n) + strings.Repeat(" ", 5<<20)
	for _, body := range []string{`{"action": "opened"}`, `not JSON`, tooBig} {
		if status, code := f.deliver(sign(f.secret, body), "pull_request", body); status != http.StatusBadRequest || code != "bad_request" {
			t.Errorf("%.20s: got %d %s, want 400 bad_request", body, status, code)
		}
	}
	if r := f.record(n); r != nil {
		t.Fatalf("a bad delivery saved record %+v", r)
	}
}
