package planning_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestCreateValidates(t *testing.T) {
	f := setup(t)
	for _, bad := range []struct {
		body any
		code string
	}{
		{map[string]string{"title": " ", "description": "CSV."}, "invalid_title"},
		{map[string]string{"title": strings.Repeat("é", 257), "description": "CSV."}, "invalid_title"},
		{map[string]string{"title": "Exports", "description": ""}, "invalid_description"},
		{map[string]string{"title": "Exports", "description": "a\x00b"}, "invalid_description"},
		{map[string]string{"title": "Exports", "description": strings.Repeat("é", 10001)}, "invalid_description"},
		{"{", "bad_request"},
	} {
		f.Do("POST", f.path("/plans"), bad.body, f.ada).Expect(http.StatusBadRequest, bad.code)
	}
	f.start(strings.Repeat("é", 256), strings.Repeat("é", 10000))
}

func TestCreateNeedsAPlanningModel(t *testing.T) {
	f := setup(t)
	bob := f.member("bob", "push")
	body := map[string]string{"title": "Exports", "description": "CSV."}
	f.Do("POST", f.path("/plans"), body, bob).Expect(http.StatusConflict, "planning_not_set_up")
	f.Do("PUT", "/api/me/planning", map[string]any{"providerId": nil, "model": nil, "review": false}, bob).Expect(http.StatusOK, "")
	f.Do("POST", f.path("/plans"), body, bob).Expect(http.StatusConflict, "planning_not_set_up")
	f.Do("PUT", "/api/me/planning", map[string]any{"providerId": f.addProvider(bob), "model": "test-chat"}, bob).Expect(http.StatusOK, "")
	f.Do("POST", f.path("/plans"), body, bob).Expect(http.StatusCreated, "")
}
