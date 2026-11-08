package planning_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/llm/llmtest"
)

// planningSettings is GET /api/me/planning's answer as session, as JSON text.
func (f *fixture) planningSettings(session *http.Cookie) string {
	f.t.Helper()
	res := f.Do("GET", "/api/me/planning", nil, session)
	res.Expect(http.StatusOK, "")
	return strings.TrimSpace(string(res.Body))
}

// none is the settings JSON of an agent with no model.
const none = `{"providerId":null,"model":null,"instructions":""}`

func TestSaveSettings(t *testing.T) {
	f := setup(t)
	chat := fmt.Sprintf(`{"providerId":%d,"model":"test-chat","instructions":""}`, f.provider)
	want := `{"review":true,"interviewer":` + chat + `,"taskAgent":` + chat + `}`
	if got := f.planningSettings(f.ada); got != want {
		t.Fatalf("settings %s, want %s", got, want)
	}

	res := f.Do("PUT", "/api/me/planning", map[string]any{
		"review":      false,
		"interviewer": map[string]any{"providerId": f.provider, "model": llmtest.ChatModel, "instructions": "  Ask in French.\n"},
		"taskAgent":   map[string]any{"providerId": f.provider, "model": llmtest.DecisionModel, "instructions": "Keep tasks tiny."},
	}, f.ada)
	res.Expect(http.StatusOK, "")
	want = fmt.Sprintf(`{"review":false,"interviewer":{"providerId":%d,"model":"test-chat","instructions":"Ask in French."},`+
		`"taskAgent":{"providerId":%d,"model":"test-decisions","instructions":"Keep tasks tiny."}}`, f.provider, f.provider)
	if got := strings.TrimSpace(string(res.Body)); got != want || f.planningSettings(f.ada) != want {
		t.Fatalf("saved %s, shown %s, want %s", got, f.planningSettings(f.ada), want)
	}

	// An agent left out, as in the old flat shape, is refused rather than wiped.
	for _, body := range []map[string]any{{}, {"review": true}, {"providerId": f.provider, "model": llmtest.ChatModel},
		{"interviewer": map[string]any{}}, {"taskAgent": map[string]any{}}, {"interviewer": nil, "taskAgent": map[string]any{}}} {
		f.Do("PUT", "/api/me/planning", body, f.ada).Expect(http.StatusBadRequest, "bad_request")
	}
	if got := f.planningSettings(f.ada); got != want {
		t.Fatalf("settings %s after refused saves, want %s", got, want)
	}
	// An empty agent is none.
	f.Do("PUT", "/api/me/planning", map[string]any{"interviewer": map[string]any{}, "taskAgent": map[string]any{}}, f.ada).
		Expect(http.StatusOK, "")
	if got, want := f.planningSettings(f.ada), `{"review":true,"interviewer":`+none+`,"taskAgent":`+none+`}`; got != want {
		t.Fatalf("cleared settings %s, want %s", got, want)
	}
}

func TestSaveSettingsValidates(t *testing.T) {
	f := setup(t)
	bob := f.member("bob", "push")
	bobs := f.addProvider(bob)
	for _, bad := range []struct {
		agent map[string]any
		code  string
	}{
		{map[string]any{"providerId": bobs, "model": "test-chat"}, "invalid_provider"},
		{map[string]any{"providerId": 999, "model": "test-chat"}, "invalid_provider"},
		{map[string]any{"model": "test-chat"}, "invalid_provider"},
		{map[string]any{"providerId": f.provider}, "invalid_model"},
		{map[string]any{"providerId": f.provider, "model": "gpt-5"}, "invalid_model"},
		{map[string]any{"instructions": strings.Repeat("é", 4001)}, "invalid_instructions"},
		{map[string]any{"instructions": "Be brief.\x00"}, "invalid_instructions"},
	} {
		for _, name := range []string{"interviewer", "taskAgent"} {
			body := map[string]any{"interviewer": map[string]any{}, "taskAgent": map[string]any{}, name: bad.agent}
			f.Do("PUT", "/api/me/planning", body, f.ada).Expect(http.StatusBadRequest, bad.code)
		}
	}
	f.Do("PUT", "/api/me/planning", "{", f.ada).Expect(http.StatusBadRequest, "bad_request")
	chat := fmt.Sprintf(`{"providerId":%d,"model":"test-chat","instructions":""}`, f.provider)
	if got, want := f.planningSettings(f.ada), `{"review":true,"interviewer":`+chat+`,"taskAgent":`+chat+`}`; got != want {
		t.Fatalf("settings %s after refused saves, want %s", got, want)
	}
	// The most characters are fine.
	f.Do("PUT", "/api/me/planning", map[string]any{"interviewer": map[string]any{}, "taskAgent": map[string]any{"instructions": strings.Repeat("é", 4000)}}, f.ada).
		Expect(http.StatusOK, "")
	f.Do("PUT", "/api/me/planning", map[string]any{}).Expect(http.StatusUnauthorized, "unauthorized")
}

// A provider either agent uses cannot be deleted until the settings stop using it.
func TestSettingsKeepTheirProvider(t *testing.T) {
	f := setup(t)
	path := fmt.Sprintf("/api/providers/%d", f.provider)
	for _, name := range []string{"interviewer", "taskAgent"} {
		body := map[string]any{"interviewer": map[string]any{}, "taskAgent": map[string]any{}, name: map[string]any{"providerId": f.provider, "model": llmtest.ChatModel}}
		f.Do("PUT", "/api/me/planning", body, f.ada).
			Expect(http.StatusOK, "")
		f.Do("DELETE", path, nil, f.ada).Expect(http.StatusConflict, "provider_in_use")
	}
	f.Do("PUT", "/api/me/planning", agents(nil, nil, true), f.ada).Expect(http.StatusOK, "")
	f.Do("DELETE", path, nil, f.ada).Expect(http.StatusNoContent, "")
}
