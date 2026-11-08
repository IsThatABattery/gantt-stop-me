package automation_test

import (
	"bytes"
	"context"
	"maps"
	"net/http"
	"strings"
	"testing"
)

func TestEnable(t *testing.T) {
	f := setup(t)
	res := f.enable(f.ada, f.provider)
	res.Expect(http.StatusOK, "")
	if shown := f.Do("GET", f.path("/automation"), nil, f.ada); !bytes.Equal(res.Body, shown.Body) {
		t.Fatalf("PUT answered %s, GET answers %s", res.Body, shown.Body)
	}
	var owner int64
	var decision, chat string
	if err := f.DB.QueryRow(context.Background(), `select owner_id, decision_model, chat_model from project_automation`).
		Scan(&owner, &decision, &chat); err != nil {
		t.Fatal(err)
	}
	if owner != f.adaID || decision != "luna" || chat != "chat" {
		t.Fatalf("stored owner %d, models %q %q", owner, decision, chat)
	}
}

func TestEnableTwoProviders(t *testing.T) {
	f := setup(t)
	other := f.addProvider(f.adaID, "gpt")
	res := f.Do("PUT", f.path("/automation"), map[string]any{
		"decisionProviderId": f.provider, "decisionModel": "luna", "chatProviderId": other, "chatModel": "gpt",
	}, f.ada)
	res.Expect(http.StatusOK, "")
	var s settings
	res.JSON(&s)
	if s.DecisionProviderID != f.provider || s.ChatProviderID != other || s.ChatModel != "gpt" {
		t.Fatalf("settings = %s", res.Body)
	}
}

func TestEnableChecksProvidersAndModels(t *testing.T) {
	f := setup(t)
	_, bobID, _ := f.member("bob", "push")
	bobs := f.addProvider(bobID, "luna", "chat")
	for _, c := range []struct {
		name                string
		decision, chat      int64
		decModel, chatModel string
		code                string
	}{
		{"unknown decision provider", 999, f.provider, "luna", "chat", "invalid_provider"},
		{"unknown chat provider", f.provider, 999, "luna", "chat", "invalid_provider"},
		{"another user's decision provider", bobs, f.provider, "luna", "chat", "invalid_provider"},
		{"another user's chat provider", f.provider, bobs, "luna", "chat", "invalid_provider"},
		{"decision model not chosen", f.provider, f.provider, "gpt", "chat", "invalid_model"},
		{"chat model not chosen", f.provider, f.provider, "luna", "", "invalid_model"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f.Do("PUT", f.path("/automation"), map[string]any{
				"decisionProviderId": c.decision, "decisionModel": c.decModel, "chatProviderId": c.chat, "chatModel": c.chatModel,
			}, f.ada).Expect(http.StatusBadRequest, c.code)
		})
	}
	if s := f.Do("GET", f.path("/automation"), nil, f.ada); string(s.Body) != "{\"enabled\":false}\n" {
		t.Fatalf("a refused PUT turned automation on: %s", s.Body)
	}
	f.Do("PUT", f.path("/automation"), "{", f.ada).Expect(http.StatusBadRequest, "bad_request")
}

func TestEnableInstructions(t *testing.T) {
	f := setup(t)
	body := map[string]any{"decisionProviderId": f.provider, "decisionModel": "luna", "chatProviderId": f.provider, "chatModel": "chat",
		"decisionInstructions": " Prefer backend.\n", "chatInstructions": "Use curl."}
	res := f.Do("PUT", f.path("/automation"), body, f.ada)
	res.Expect(http.StatusOK, "")
	var s settings
	res.JSON(&s)
	if s.DecisionInstructions != "Prefer backend." || s.ChatInstructions != "Use curl." {
		t.Fatalf("settings = %s", res.Body)
	}
	if s = f.show(f.ada); s.DecisionInstructions != "Prefer backend." || s.ChatInstructions != "Use curl." {
		t.Fatalf("shown %+v", s)
	}
	for _, bad := range []map[string]any{
		{"decisionInstructions": strings.Repeat("é", 4001)}, {"chatInstructions": strings.Repeat("é", 4001)},
		{"decisionInstructions": "a\x00"}, {"chatInstructions": "a\x00"},
	} {
		maps.Copy(bad, map[string]any{"decisionProviderId": f.provider, "decisionModel": "luna", "chatProviderId": f.provider, "chatModel": "chat"})
		f.Do("PUT", f.path("/automation"), bad, f.ada).Expect(http.StatusBadRequest, "invalid_instructions")
	}
	// A PUT replaces them: left out is none.
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	if s = f.show(f.ada); s.DecisionInstructions != "" || s.ChatInstructions != "" {
		t.Fatalf("after a PUT without instructions %+v", s)
	}
}

func TestEnableTakesOver(t *testing.T) {
	f := setup(t)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	f.Do("POST", f.path("/automation/webhook"), nil, f.ada).Expect(http.StatusOK, "")
	f.exec(`update project_automation set paused_reason = 'ada lost write access to the repository.', pulls_etag = '"abc"'`)
	var sealed []byte
	if err := f.DB.QueryRow(context.Background(), `select webhook_secret_enc from project_automation`).Scan(&sealed); err != nil {
		t.Fatal(err)
	}

	carol, carolID, _ := f.member("carol", "maintain")
	res := f.enable(carol, f.addProvider(carolID, "luna", "chat"))
	res.Expect(http.StatusOK, "")
	var s settings
	res.JSON(&s)
	if s.Owner == nil || s.Owner.ID != carolID || *s.Owner.Login != "carol" || s.PausedReason != nil || !s.WebhookSecretSet {
		t.Fatalf("after takeover: %s", res.Body)
	}
	var kept []byte
	var etag string
	if err := f.DB.QueryRow(context.Background(), `select webhook_secret_enc, pulls_etag from project_automation`).Scan(&kept, &etag); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(kept, sealed) || etag != `"abc"` {
		t.Fatalf("takeover changed the webhook secret or the ETag %q", etag)
	}
}

func TestEnableNeedsWriteAccess(t *testing.T) {
	f := setup(t)
	bob, bobID, _ := f.member("bob", "triage")
	eve, eveID, _ := f.member("eve", "")
	f.enable(bob, f.addProvider(bobID, "luna", "chat")).Expect(http.StatusForbidden, "forbidden")
	f.enable(eve, f.addProvider(eveID, "luna", "chat")).Expect(http.StatusNotFound, "not_found")
	f.enable(nil, f.provider).Expect(http.StatusUnauthorized, "unauthorized")
	f.Do("PUT", "/api/projects/999/automation", map[string]any{}, f.ada).Expect(http.StatusNotFound, "not_found")
}
