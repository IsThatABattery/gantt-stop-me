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

func TestSaveSettings(t *testing.T) {
	f := setup(t)
	want := fmt.Sprintf(`{"providerId":%d,"model":"test-chat","review":true}`, f.provider)
	if got := f.planningSettings(f.ada); got != want {
		t.Fatalf("settings %s, want %s", got, want)
	}

	res := f.Do("PUT", "/api/me/planning", map[string]any{"providerId": f.provider, "model": llmtest.DecisionModel, "review": false}, f.ada)
	res.Expect(http.StatusOK, "")
	want = fmt.Sprintf(`{"providerId":%d,"model":"test-decisions","review":false}`, f.provider)
	if got := strings.TrimSpace(string(res.Body)); got != want || f.planningSettings(f.ada) != want {
		t.Fatalf("saved %s, shown %s, want %s", got, f.planningSettings(f.ada), want)
	}
	f.Do("PUT", "/api/me/planning", map[string]any{"providerId": nil, "model": nil}, f.ada).Expect(http.StatusOK, "")
	if got := f.planningSettings(f.ada); got != `{"providerId":null,"model":null,"review":true}` {
		t.Fatalf("cleared settings %s", got)
	}
}

func TestSaveSettingsValidates(t *testing.T) {
	f := setup(t)
	bob := f.member("bob", "push")
	bobs := f.addProvider(bob)
	for _, bad := range []struct {
		body any
		code string
	}{
		{map[string]any{"providerId": bobs, "model": "test-chat"}, "invalid_provider"},
		{map[string]any{"providerId": 999, "model": "test-chat"}, "invalid_provider"},
		{map[string]any{"model": "test-chat"}, "invalid_provider"},
		{map[string]any{"providerId": f.provider}, "invalid_model"},
		{map[string]any{"providerId": f.provider, "model": "gpt-5"}, "invalid_model"},
		{"{", "bad_request"},
	} {
		f.Do("PUT", "/api/me/planning", bad.body, f.ada).Expect(http.StatusBadRequest, bad.code)
	}
	if got, want := f.planningSettings(f.ada), fmt.Sprintf(`{"providerId":%d,"model":"test-chat","review":true}`, f.provider); got != want {
		t.Fatalf("settings %s after refused saves, want %s", got, want)
	}
	f.Do("PUT", "/api/me/planning", map[string]any{}).Expect(http.StatusUnauthorized, "unauthorized")
}

// The provider planning uses cannot be deleted until the settings stop using it.
func TestSettingsKeepTheirProvider(t *testing.T) {
	f := setup(t)
	path := fmt.Sprintf("/api/providers/%d", f.provider)
	f.Do("DELETE", path, nil, f.ada).Expect(http.StatusConflict, "provider_in_use")
	f.Do("PUT", "/api/me/planning", map[string]any{"providerId": nil, "model": nil}, f.ada).Expect(http.StatusOK, "")
	f.Do("DELETE", path, nil, f.ada).Expect(http.StatusNoContent, "")
}
