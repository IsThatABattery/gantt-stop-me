package settings_test

import (
	"net/http"
	"slices"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestDelete(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	p := create(t, env, session, "OpenAI", up.URL, key)
	create(t, env, session, "Other", up.URL, key)

	env.Do("DELETE", providerPath(p.ID), nil, session).Expect(http.StatusNoContent, "")
	if names := listNames(t, env, session); !slices.Equal(names, []string{"Other"}) {
		t.Fatalf("providers after delete %v", names)
	}
	env.Do("DELETE", providerPath(p.ID), nil, session).Expect(http.StatusNotFound, "not_found")
}

func TestDeleteOtherUsersProvider(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	p := create(t, env, ada, "OpenAI", up.URL, key)

	env.Do("DELETE", providerPath(p.ID), nil, bob).Expect(http.StatusNotFound, "not_found")
	env.Do("DELETE", "/api/providers/nope", nil, ada).Expect(http.StatusNotFound, "not_found")
	if n := countProviders(t, env); n != 1 {
		t.Fatalf("%d providers left, want ada's one", n)
	}
}

func TestDeleteNeedsSession(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	p := create(t, env, session, "OpenAI", up.URL, key)

	env.Do("DELETE", providerPath(p.ID), nil).Expect(http.StatusUnauthorized, "unauthorized")
	if n := countProviders(t, env); n != 1 {
		t.Fatalf("%d providers left", n)
	}
}
