package automation_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestShowWhenOff(t *testing.T) {
	f := setup(t)
	res := f.Do("GET", f.path("/automation"), nil, f.ada)
	res.Expect(http.StatusOK, "")
	if got := strings.TrimSpace(string(res.Body)); got != `{"enabled":false}` {
		t.Fatalf("body = %s", got)
	}
}

func TestShowToReaders(t *testing.T) {
	f := setup(t)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	bob, _, _ := f.member("bob", "pull")

	res := f.Do("GET", f.path("/automation"), nil, bob)
	res.Expect(http.StatusOK, "")
	if strings.Contains(string(res.Body), "ada@example.com") {
		t.Fatalf("members see the owner's email: %s", res.Body)
	}
	var s settings
	res.JSON(&s)
	if !s.Enabled || s.Owner == nil || s.Owner.ID != f.adaID || s.Owner.Login == nil || *s.Owner.Login != "ada" ||
		s.DecisionProviderID != f.provider || s.DecisionModel != "luna" || s.ChatProviderID != f.provider || s.ChatModel != "chat" ||
		s.PausedReason != nil || s.WebhookSecretSet ||
		s.PayloadURL != fmt.Sprintf("%s/api/github/webhook/%d", apptest.PublicURL, f.project) {
		t.Fatalf("settings = %s", res.Body)
	}
}

func TestShowPaused(t *testing.T) {
	f := setup(t)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	f.exec(`update project_automation set paused_reason = 'ada lost write access to the repository.'`)
	if s := f.show(f.ada); s.PausedReason == nil || *s.PausedReason != "ada lost write access to the repository." {
		t.Fatalf("pausedReason = %v", s.PausedReason)
	}
}

func TestShowAfterOwnerUnlinks(t *testing.T) {
	f := setup(t)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	bob, _, _ := f.member("bob", "pull")
	f.Do("DELETE", "/api/github", nil, f.ada).Expect(http.StatusNoContent, "")
	if s := f.show(bob); s.Owner == nil || s.Owner.ID != f.adaID || s.Owner.Login != nil {
		t.Fatalf("owner = %+v, want ada's id with no login", s.Owner)
	}
}

func TestShowNeedsAccess(t *testing.T) {
	f := setup(t)
	eve, _, _ := f.member("eve", "")
	f.Do("GET", f.path("/automation"), nil, eve).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", "/api/projects/999/automation", nil, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path("/automation"), nil).Expect(http.StatusUnauthorized, "unauthorized")
}
