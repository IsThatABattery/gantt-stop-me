package automation_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

// webhookSecret generates a secret as session and returns it, checking the answer's payload URL.
func (f *fixture) webhookSecret(session *http.Cookie) string {
	f.t.Helper()
	res := f.Do("POST", f.path("/automation/webhook"), nil, session)
	res.Expect(http.StatusOK, "")
	var out struct{ Secret, PayloadURL string }
	res.JSON(&out)
	if want := fmt.Sprintf("%s/api/github/webhook/%d", apptest.PublicURL, f.project); out.PayloadURL != want {
		f.t.Fatalf("payloadUrl = %q, want %q", out.PayloadURL, want)
	}
	if b, err := hex.DecodeString(out.Secret); err != nil || len(b) != 32 {
		f.t.Fatalf("secret %q is not 32 hex-encoded bytes", out.Secret)
	}
	return out.Secret
}

// storedSecret opens the project's stored webhook secret.
func (f *fixture) storedSecret() string {
	f.t.Helper()
	var sealed []byte
	if err := f.DB.QueryRow(context.Background(), `select webhook_secret_enc from project_automation where project_id = $1`, f.project).
		Scan(&sealed); err != nil {
		f.t.Fatal(err)
	}
	plain, err := f.Config.Secrets.Open(sealed)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(plain)
}

func TestWebhookSecret(t *testing.T) {
	f := setup(t)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	carol, _, _ := f.member("carol", "push")

	first := f.webhookSecret(f.ada)
	if got := f.storedSecret(); got != first {
		t.Fatalf("stored secret opens to %q, answered %q", got, first)
	}
	if !f.show(f.ada).WebhookSecretSet {
		t.Fatal("GET says no webhook secret is set")
	}
	second := f.webhookSecret(carol) // any write member, not only the owner
	if second == first || f.storedSecret() != second {
		t.Fatalf("a new secret %q did not replace %q", second, first)
	}
}

func TestWebhookSecretNeedsAutomation(t *testing.T) {
	f := setup(t)
	f.Do("POST", f.path("/automation/webhook"), nil, f.ada).Expect(http.StatusConflict, "automation_off")
}

func TestWebhookSecretNeedsWriteAccess(t *testing.T) {
	f := setup(t)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	bob, _, _ := f.member("bob", "pull")
	eve, _, _ := f.member("eve", "")
	f.Do("POST", f.path("/automation/webhook"), nil, bob).Expect(http.StatusForbidden, "forbidden")
	f.Do("POST", f.path("/automation/webhook"), nil, eve).Expect(http.StatusNotFound, "not_found")
	f.Do("POST", f.path("/automation/webhook"), nil).Expect(http.StatusUnauthorized, "unauthorized")
	f.Do("POST", "/api/projects/999/automation/webhook", nil, f.ada).Expect(http.StatusNotFound, "not_found")
	if f.show(f.ada).WebhookSecretSet {
		t.Fatal("a refused POST set a secret")
	}
}
