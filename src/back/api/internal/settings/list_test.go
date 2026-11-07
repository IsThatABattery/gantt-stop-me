package settings_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func listNames(t *testing.T, env *apptest.Env, session *http.Cookie) []string {
	t.Helper()
	res := env.Do("GET", "/api/providers", nil, session)
	res.Expect(http.StatusOK, "")
	var body struct{ Providers []providerJSON }
	res.JSON(&body)
	var names []string
	for _, p := range body.Providers {
		names = append(names, p.Name)
	}
	return names
}

func TestList(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o", "o3")
	for _, name := range []string{"Charlie", "Alpha", "Bravo"} {
		create(t, env, ada, name, up.URL, key, "o3")
	}
	create(t, env, bob, "Delta", up.URL, key)

	res := env.Do("GET", "/api/providers", nil, ada)
	res.Expect(http.StatusOK, "")
	var body struct{ Providers []providerJSON }
	res.JSON(&body)
	if len(body.Providers) != 3 {
		t.Fatalf("listed %+v", body.Providers)
	}
	if p := body.Providers[0]; p.ID == 0 || p.Name != "Alpha" || p.BaseURL != up.URL || p.KeyLast4 != "1234" ||
		!slices.Equal(p.Models, []string{"o3"}) || p.AvailableModels != nil {
		t.Fatalf("first provider %+v", p)
	}
	if strings.Contains(string(res.Body), key) {
		t.Fatalf("list %s leaks the key", res.Body)
	}

	if names := listNames(t, env, ada); !slices.Equal(names, []string{"Alpha", "Bravo", "Charlie"}) {
		t.Fatalf("ada's providers %v, want ordered by name", names)
	}
	if names := listNames(t, env, bob); !slices.Equal(names, []string{"Delta"}) {
		t.Fatalf("bob's providers %v", names)
	}
	carol := env.SignUp("carol@example.com", "correct horse")
	if res := env.Do("GET", "/api/providers", nil, carol); strings.TrimSpace(string(res.Body)) != `{"providers":[]}` {
		t.Fatalf("empty list = %s", res.Body)
	}
}

func TestListHidesShortKey(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, "abcd")

	if p := create(t, env, session, "Local", up.URL, "abcd"); p.KeyLast4 != "" {
		t.Fatalf("created keyLast4 %q, want empty for a 4-character key", p.KeyLast4)
	}
	res := env.Do("GET", "/api/providers", nil, session)
	if strings.Contains(string(res.Body), "abcd") || !strings.Contains(string(res.Body), `"keyLast4":""`) {
		t.Fatalf("list %s shows the whole key", res.Body)
	}
}

func TestListNeedsSession(t *testing.T) {
	apptest.New(t).Do("GET", "/api/providers", nil).Expect(http.StatusUnauthorized, "unauthorized")
}
