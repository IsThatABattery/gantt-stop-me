package planning_test

import (
	"net/http"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestShowSettings(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	res := env.Do("GET", "/api/me/planning", nil, session)
	res.Expect(http.StatusOK, "")
	if got := strings.TrimSpace(string(res.Body)); got != `{"providerId":null,"model":null,"review":true}` {
		t.Fatalf("unset settings %s", got)
	}
	env.Do("GET", "/api/me/planning", nil).Expect(http.StatusUnauthorized, "unauthorized")
}
