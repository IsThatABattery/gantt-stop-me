package settings_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
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
	env.Do("PATCH", providerPath(p.ID), map[string]any{"models": []string{"o3", "o3"}}, session).
		Expect(http.StatusBadRequest, "duplicate_model")
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
	env.Do("PATCH", providerPath(p.ID), map[string]string{"apiKey": "sk-te\x00st"}, session).Expect(http.StatusBadRequest, "invalid_api_key")
	// An endpoint that ignores the key, like a local Ollama, must not let an empty one through.
	open := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`)) })
	env.Do("PATCH", providerPath(p.ID), map[string]string{"baseUrl": open, "apiKey": ""}, session).
		Expect(http.StatusBadRequest, "invalid_api_key")
	if base, k, _ := stored(t, env, p.ID); base != up.URL || k != key {
		t.Fatalf("stored %q %q after rejected keys", base, k)
	}

	// A pasted trailing newline is trimmed.
	res := env.Do("PATCH", providerPath(p.ID), map[string]string{"apiKey": rotated + "\n"}, session)
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
	env.Do("PATCH", providerPath(p.ID), map[string]string{"baseUrl": closed.URL, "apiKey": key}, session).
		Expect(http.StatusBadRequest, "provider_unreachable")
	if base, _, _ := stored(t, env, p.ID); base != first.URL {
		t.Fatalf("stored base URL %q after rejected updates", base)
	}
	// The stored URL, however it is spelled, still works with the stored key.
	env.Do("PATCH", providerPath(p.ID), map[string]string{"baseUrl": first.URL + "/"}, session).Expect(http.StatusOK, "")

	res := env.Do("PATCH", providerPath(p.ID), map[string]any{"baseUrl": second.URL + "/", "apiKey": key, "models": []string{"llama3"}}, session)
	res.Expect(http.StatusOK, "")
	var got providerJSON
	res.JSON(&got)
	if got.BaseURL != second.URL || !slices.Equal(got.AvailableModels, []string{"llama3"}) {
		t.Fatalf("updated %+v", got)
	}
	if auth := second.lastAuth(); auth != "Bearer "+key {
		t.Fatalf("new endpoint saw Authorization %q, want the key sent with it", auth)
	}
	if base, _, models := stored(t, env, p.ID); base != second.URL || !slices.Equal(models, []string{"llama3"}) {
		t.Fatalf("stored %q %v", base, models)
	}
}

func TestUpdateBaseURLNeedsKey(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	other := newUpstream(t, key, "gpt-4o")
	p := create(t, env, session, "OpenAI", up.URL, key, "gpt-4o")

	env.Do("PATCH", providerPath(p.ID), map[string]string{"baseUrl": other.URL}, session).
		Expect(http.StatusBadRequest, "invalid_api_key")
	if auth := other.lastAuth(); auth != "" {
		t.Fatalf("the stored key went to a new base URL as %q", auth)
	}
	if base, k, _ := stored(t, env, p.ID); base != up.URL || k != key {
		t.Fatalf("stored %q %q", base, k)
	}
}

// gate holds one request inside a fake endpoint's model listing.
type gate struct {
	armed            atomic.Bool
	entered, release chan struct{}
}

func newGate() *gate { return &gate{entered: make(chan struct{}), release: make(chan struct{})} }

// hold, called by the endpoint, holds the first request after overlap arms the gate until overlap releases it.
func (g *gate) hold(r *http.Request) {
	if g.armed.CompareAndSwap(true, false) {
		close(g.entered)
		select {
		case <-g.release:
		case <-r.Context().Done():
		}
	}
}

// overlap sends the slow PATCH, sends fast while slow is held in its listing call, then lets slow finish.
func (g *gate) overlap(t *testing.T, env *apptest.Env, session *http.Cookie, path string, slow, fast any) {
	t.Helper()
	g.armed.Store(true)
	done := make(chan *apptest.Response)
	go func() { done <- env.Do("PATCH", path, slow, session) }()
	select {
	case <-g.entered:
	case res := <-done:
		t.Fatalf("slow PATCH answered %d %s without reaching the endpoint", res.Status, res.Body)
	}
	env.Do("PATCH", path, fast, session).Expect(http.StatusOK, "")
	close(g.release)
	(<-done).Expect(http.StatusOK, "")
}

func TestUpdateRefreshKeepsConcurrentKeyRotation(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	g := newGate()
	base := serve(t, func(w http.ResponseWriter, r *http.Request) {
		g.hold(r)
		w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`))
	})
	p := create(t, env, session, "OpenAI", base, key, "gpt-4o")
	const rotated = "sk-test-wxyz5678"

	g.overlap(t, env, session, providerPath(p.ID), map[string]any{}, map[string]string{"apiKey": rotated})
	if _, k, _ := stored(t, env, p.ID); k != rotated {
		t.Fatalf("stored key %q: the overlapping refresh reverted the rotation", k)
	}
}

func TestUpdateModelsKeepsConcurrentBaseURLChange(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	g := newGate()
	held := serve(t, func(w http.ResponseWriter, r *http.Request) {
		g.hold(r)
		w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"o3"}]}`))
	})
	second := newUpstream(t, key, "gpt-4o", "o3")
	p := create(t, env, session, "OpenAI", held, key, "gpt-4o")

	g.overlap(t, env, session, providerPath(p.ID), map[string]any{"models": []string{"o3"}},
		map[string]string{"baseUrl": second.URL, "apiKey": key})
	if base, _, models := stored(t, env, p.ID); base != second.URL || !slices.Equal(models, []string{"o3"}) {
		t.Fatalf("stored %q %v, want both overlapping changes", base, models)
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

	// Only the read's ownership check stops these: a refresh writes nothing, and a new URL would get ada's key.
	bobs := newUpstream(t, "sk-bob-0000", "gpt-4o")
	env.Do("PATCH", providerPath(p.ID), map[string]any{}, bob).Expect(http.StatusNotFound, "not_found")
	env.Do("PATCH", providerPath(p.ID), map[string]any{"baseUrl": bobs.URL, "models": []string{}}, bob).
		Expect(http.StatusNotFound, "not_found")
	if auth := bobs.lastAuth(); auth != "" {
		t.Fatalf("another user's PATCH sent Authorization %q to its own base URL", auth)
	}
	env.Do("PATCH", providerPath(p.ID+1), map[string]any{}, ada).Expect(http.StatusNotFound, "not_found")
	env.Do("PATCH", "/api/providers/nope", map[string]any{}, ada).Expect(http.StatusNotFound, "not_found")
	if base, _, models := stored(t, env, p.ID); base != up.URL || !slices.Equal(models, []string{"gpt-4o"}) {
		t.Fatalf("stored %q %v", base, models)
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
