package llmtest_test

import (
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/llm/llmtest"
)

func TestRegistersAsProvider(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	f := llmtest.New(t)

	env.Do("POST", "/api/providers", map[string]any{"name": "Fake", "baseUrl": f.URL, "apiKey": "sk-wrong"}, session).
		Expect(http.StatusBadRequest, "invalid_api_key")
	env.Do("POST", "/api/providers", map[string]any{"name": "Fake", "baseUrl": f.URL, "apiKey": f.Key, "models": f.Models}, session).
		Expect(http.StatusCreated, "")
}
