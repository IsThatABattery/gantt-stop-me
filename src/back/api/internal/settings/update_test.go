package settings_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestUpdateModels(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o", "gpt-4o-mini", "o3")
	p := create(t, env, session, "OpenAI", up.URL, key, "gpt-4o")

	res := env.Do("PATCH", providerPath(p.ID), map[string]any{"models": []string{"o3", "gpt-4o-mini"}}, session)
	res.Expect(http.StatusOK, "")
	var got providerJSON
	res.JSON(&got)
	if got.ID != p.ID || got.Name != "OpenAI" || got.BaseURL != up.URL || got.KeyLast4 != "1234" ||
		!slices.Equal(got.Models, []string{"o3", "gpt-4o-mini"}) || !slices.Equal(got.AvailableModels, []string{"gpt-4o", "gpt-4o-mini", "o3"}) {
		t.Fatalf("updated %+v", got)
	}

	env.Do("PATCH", providerPath(p.ID), map[string]any{"models": []string{"o3", "gpt-5"}}, session).
		Expect(http.StatusBadRequest, "unknown_model")
	if _, _, models := stored(t, env, p.ID); !slices.Equal(models, []string{"o3", "gpt-4o-mini"}) {
		t.Fatalf("stored models %v after a rejected update", models)
	}

	env.Do("PATCH", providerPath(p.ID), map[string]any{"models": []string{}}, session).Expect(http.StatusOK, "")
	if _, _, models := stored(t, env, p.ID); len(models) != 0 {
		t.Fatalf("stored models %v, want none", models)
	}
}

func TestUpdateKey(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	p := create(t, env, session, "OpenAI", up.URL, key, "gpt-4o")
	const rotated = "sk-test-wxyz5678"
	up.set(rotated, "gpt-4o")

	env.Do("PATCH", providerPath(p.ID), map[string]string{"apiKey": "sk-wrong"}, session).Expect(http.StatusBadRequest, "invalid_api_key")
	env.Do("PATCH", providerPath(p.ID), map[string]string{"apiKey": ""}, session).Expect(http.StatusBadRequest, "invalid_api_key")
	if _, k, _ := stored(t, env, p.ID); k != key {
		t.Fatal("a rejected key was saved")
	}

	res := env.Do("PATCH", providerPath(p.ID), map[string]string{"apiKey": rotated}, session)
	res.Expect(http.StatusOK, "")
	var got providerJSON
	res.JSON(&got)
	if got.KeyLast4 != "5678" || !slices.Equal(got.Models, []string{"gpt-4o"}) || strings.Contains(string(res.Body), rotated) {
		t.Fatalf("updated %s", res.Body)
	}
	if auth := up.lastAuth(); auth != "Bearer "+rotated {
		t.Fatalf("endpoint saw Authorization %q", auth)
	}
	if _, k, _ := stored(t, env, p.ID); k != rotated {
		t.Fatalf("stored key %q, want the new one", k)
	}
}

func TestUpdateBaseURL(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	first := newUpstream(t, key, "gpt-4o")
	second := newUpstream(t, key, "llama3")
	p := create(t, env, session, "Local", first.URL, key, "gpt-4o")
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	env.Do("PATCH", providerPath(p.ID), map[string]string{"baseUrl": "localhost:11434"}, session).
		Expect(http.StatusBadRequest, "invalid_base_url")
	env.Do("PATCH", providerPath(p.ID), map[string]string{"baseUrl": closed.URL}, session).
		Expect(http.StatusBadRequest, "provider_unreachable")
	if base, _, _ := stored(t, env, p.ID); base != first.URL {
		t.Fatalf("stored base URL %q after rejected updates", base)
	}

	res := env.Do("PATCH", providerPath(p.ID), map[string]any{"baseUrl": second.URL + "/", "models": []string{"llama3"}}, session)
	res.Expect(http.StatusOK, "")
	var got providerJSON
	res.JSON(&got)
	if got.BaseURL != second.URL || !slices.Equal(got.AvailableModels, []string{"llama3"}) {
		t.Fatalf("updated %+v", got)
	}
	if auth := second.lastAuth(); auth != "Bearer "+key {
		t.Fatalf("new endpoint saw Authorization %q, want the stored key", auth)
	}
	if base, _, models := stored(t, env, p.ID); base != second.URL || !slices.Equal(models, []string{"llama3"}) {
		t.Fatalf("stored %q %v", base, models)
	}
}

func TestUpdateRefreshesListing(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	p := create(t, env, session, "OpenAI", up.URL, key, "gpt-4o")

	refresh := func() providerJSON {
		t.Helper()
		res := env.Do("PATCH", providerPath(p.ID), map[string]any{}, session)
		res.Expect(http.StatusOK, "")
		var got providerJSON
		res.JSON(&got)
		return got
	}
	up.set(key, "gpt-4o", "gpt-5")
	if got := refresh(); !slices.Equal(got.Models, []string{"gpt-4o"}) || !slices.Equal(got.AvailableModels, []string{"gpt-4o", "gpt-5"}) {
		t.Fatalf("refreshed %+v", got)
	}
	// A chosen model the endpoint dropped must not block the refresh that lets the user replace it.
	up.set(key, "gpt-5")
	if got := refresh(); !slices.Equal(got.Models, []string{"gpt-4o"}) || !slices.Equal(got.AvailableModels, []string{"gpt-5"}) {
		t.Fatalf("refreshed %+v", got)
	}
}

func TestUpdateOtherUsersProvider(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	p := create(t, env, ada, "OpenAI", up.URL, key, "gpt-4o")

	env.Do("PATCH", providerPath(p.ID), map[string]any{"models": []string{}}, bob).Expect(http.StatusNotFound, "not_found")
	env.Do("PATCH", providerPath(p.ID+1), map[string]any{}, ada).Expect(http.StatusNotFound, "not_found")
	env.Do("PATCH", "/api/providers/nope", map[string]any{}, ada).Expect(http.StatusNotFound, "not_found")
	if _, _, models := stored(t, env, p.ID); !slices.Equal(models, []string{"gpt-4o"}) {
		t.Fatalf("stored models %v", models)
	}
}

func TestUpdateNeedsSession(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	p := create(t, env, session, "OpenAI", up.URL, key, "gpt-4o")

	env.Do("PATCH", providerPath(p.ID), map[string]any{"models": []string{}}).Expect(http.StatusUnauthorized, "unauthorized")
	env.Do("PATCH", providerPath(p.ID), "{", session).Expect(http.StatusBadRequest, "bad_request")
	if _, _, models := stored(t, env, p.ID); !slices.Equal(models, []string{"gpt-4o"}) {
		t.Fatalf("stored models %v", models)
	}
}
