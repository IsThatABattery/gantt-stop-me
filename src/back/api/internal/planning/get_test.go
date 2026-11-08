package planning_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.llm.Reply(reply(ask("Who exports?", "Admins")))
	f.work()

	res := f.Do("GET", f.path("/plans/%d", pl.ID), nil, f.member("bob", "pull"))
	got := decode(t, res, http.StatusOK)
	if got.ID != pl.ID || len(got.Messages) != 1 || got.Messages[0].Options[0] != "Admins" {
		t.Fatalf("plan %+v", got)
	}
	for _, field := range []string{`"pending":null`, `"error":null`, `"feedback":null`, `"expectedQuestions":`, `"createdAt":`, `"updatedAt":`, `"state":"not_yet"`} {
		if !strings.Contains(string(res.Body), field) {
			t.Errorf("plan %s lacks %s", res.Body, field)
		}
	}
	f.Do("GET", f.path("/plans/999"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path("/plans/nope"), nil, f.ada).Expect(http.StatusNotFound, "not_found")

	// Another project's path does not reach the plan.
	f.GitHub.AddRepo("octo", "other").Grant(f.token, "push")
	path := fmt.Sprintf("/api/projects/%d/plans/%d", f.AddProject(f.ada, "octo", "other"), pl.ID)
	f.Do("GET", path, nil, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("POST", path+"/draft", nil, f.ada).Expect(http.StatusNotFound, "not_found")
}
