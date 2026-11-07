package settings_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/apptest"
)

const key = "sk-test-abcd1234"

// upstream is a fake OpenAI-compatible endpoint that accepts one API key.
type upstream struct {
	URL    string // base URL to register, ending in /v1
	mu     sync.Mutex
	key    string
	models []string
	auth   string
}

func newUpstream(t *testing.T, key string, models ...string) *upstream {
	u := &upstream{key: key, models: models}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.auth = r.Header.Get("Authorization")
		switch {
		case r.URL.Path != "/v1/models":
			http.NotFound(w, r)
		case u.auth != "Bearer "+u.key:
			http.Error(w, `{"error":{"message":"Incorrect API key"}}`, http.StatusUnauthorized)
		default:
			data := []map[string]string{}
			for _, m := range u.models {
				data = append(data, map[string]string{"id": m, "object": "model"})
			}
			json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
		}
	}))
	t.Cleanup(srv.Close)
	u.URL = srv.URL + "/v1"
	return u
}

// set changes the key the endpoint accepts and the models it lists.
func (u *upstream) set(key string, models ...string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.key, u.models = key, models
}

// lastAuth is the Authorization header of the last request the endpoint received.
func (u *upstream) lastAuth() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.auth
}

// serve runs h as a provider endpoint and returns its base URL.
func serve(t *testing.T, h http.HandlerFunc) string {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

type providerJSON struct {
	ID              int64
	Name            string
	BaseURL         string
	KeyLast4        string
	Models          []string
	AvailableModels []string
}

func providerPath(id int64) string { return fmt.Sprintf("/api/providers/%d", id) }

// create adds a provider through the API and fails the test unless it answers 201.
func create(t *testing.T, env *apptest.Env, session *http.Cookie, name, baseURL, apiKey string, models ...string) providerJSON {
	t.Helper()
	res := env.Do("POST", "/api/providers", map[string]any{"name": name, "baseUrl": baseURL, "apiKey": apiKey, "models": models}, session)
	res.Expect(http.StatusCreated, "")
	var p providerJSON
	res.JSON(&p)
	return p
}

// stored reads a provider's row and decrypts its key.
func stored(t *testing.T, env *apptest.Env, id int64) (baseURL, key string, models []string) {
	t.Helper()
	var keyEnc []byte
	err := env.DB.QueryRow(context.Background(), `select base_url, key_enc, models from providers where id = $1`, id).
		Scan(&baseURL, &keyEnc, &models)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := env.Config.Secrets.Open(keyEnc)
	if err != nil {
		t.Fatal(err)
	}
	return baseURL, string(plain), models
}

func countProviders(t *testing.T, env *apptest.Env) int {
	t.Helper()
	var n int
	if err := env.DB.QueryRow(context.Background(), `select count(*) from providers`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreate(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o", "gpt-4o-mini", "o3")

	res := env.Do("POST", "/api/providers", map[string]any{
		"name": "  OpenAI  ", "baseUrl": up.URL + "/", "apiKey": key, "models": []string{"o3", "gpt-4o"},
	}, session)
	res.Expect(http.StatusCreated, "")
	var p providerJSON
	res.JSON(&p)
	if p.ID == 0 || p.Name != "OpenAI" || p.BaseURL != up.URL || p.KeyLast4 != "1234" ||
		!slices.Equal(p.Models, []string{"o3", "gpt-4o"}) || !slices.Equal(p.AvailableModels, []string{"gpt-4o", "gpt-4o-mini", "o3"}) {
		t.Fatalf("created %+v", p)
	}
	body := string(res.Body)
	if strings.Contains(body, key) || !strings.Contains(body, `"baseUrl":`) || !strings.Contains(body, `"keyLast4":"1234"`) ||
		!strings.Contains(body, `"availableModels":`) {
		t.Fatalf("response %s leaks the key or is not camelCase", body)
	}
	if got := up.lastAuth(); got != "Bearer "+key {
		t.Fatalf("endpoint saw Authorization %q", got)
	}

	var keyEnc []byte
	env.DB.QueryRow(context.Background(), `select key_enc from providers where id = $1`, p.ID).Scan(&keyEnc)
	if len(keyEnc) == 0 || bytes.Contains(keyEnc, []byte(key)) {
		t.Fatalf("providers.key_enc = %q, want the key encrypted", keyEnc)
	}
	if base, k, models := stored(t, env, p.ID); base != up.URL || k != key || !slices.Equal(models, []string{"o3", "gpt-4o"}) {
		t.Fatalf("stored %q %q %v", base, k, models)
	}
}

func TestCreateWithoutModels(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")

	res := env.Do("POST", "/api/providers", map[string]string{"name": "OpenAI", "baseUrl": up.URL, "apiKey": key}, session)
	res.Expect(http.StatusCreated, "")
	if !strings.Contains(string(res.Body), `"models":[]`) {
		t.Fatalf("response %s, want models []", res.Body)
	}
}

func TestCreateValidatesInput(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	input := func(name, baseURL, apiKey string) map[string]string {
		return map[string]string{"name": name, "baseUrl": baseURL, "apiKey": apiKey}
	}
	cases := []struct {
		body any
		code string
	}{
		{input("", up.URL, key), "invalid_name"},
		{input("   ", up.URL, key), "invalid_name"},
		{input(strings.Repeat("é", 101), up.URL, key), "invalid_name"},
		{input("OpenAI", "", key), "invalid_base_url"},
		{input("OpenAI", "ftp://example.com/v1", key), "invalid_base_url"},
		{input("OpenAI", "example.com/v1", key), "invalid_base_url"},
		{input("OpenAI", "/v1", key), "invalid_base_url"},
		{input("OpenAI", "http://", key), "invalid_base_url"},
		{input("OpenAI", up.URL+"?api-version=1", key), "invalid_base_url"},
		{input("OpenAI", up.URL+"#models", key), "invalid_base_url"},
		{input("OpenAI", up.URL, ""), "invalid_api_key"},
		{input("OpenAI", up.URL, " \n "), "invalid_api_key"},
		{input("OpenAI", up.URL, "sk-te\x00st-abcd1234"), "invalid_api_key"},
		{"{", "bad_request"},
	}
	for _, c := range cases {
		env.Do("POST", "/api/providers", c.body, session).Expect(http.StatusBadRequest, c.code)
	}
	if up.lastAuth() != "" {
		t.Fatal("invalid input reached the provider endpoint")
	}
	if n := countProviders(t, env); n != 0 {
		t.Fatalf("%d providers stored", n)
	}

	p := create(t, env, session, strings.Repeat("é", 100), up.URL, key)
	if p.Name != strings.Repeat("é", 100) {
		t.Fatalf("100-character name stored as %q", p.Name)
	}
}

func TestCreateTrimsKey(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")

	p := create(t, env, session, "OpenAI", up.URL, key+"\n")
	if _, k, _ := stored(t, env, p.ID); p.KeyLast4 != "1234" || k != key {
		t.Fatalf("keyLast4 %q, stored key %q", p.KeyLast4, k)
	}
}

func TestCreateRejectsInvalidKey(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	forbidden := serve(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })

	env.Do("POST", "/api/providers", map[string]string{"name": "OpenAI", "baseUrl": up.URL, "apiKey": "sk-wrong"}, session).
		Expect(http.StatusBadRequest, "invalid_api_key")
	if got := up.lastAuth(); got != "Bearer sk-wrong" {
		t.Fatalf("endpoint saw Authorization %q", got)
	}
	env.Do("POST", "/api/providers", map[string]string{"name": "OpenAI", "baseUrl": forbidden, "apiKey": key}, session).
		Expect(http.StatusBadRequest, "invalid_api_key")
	if n := countProviders(t, env); n != 0 {
		t.Fatalf("%d providers stored", n)
	}
}

func TestCreateReportsUnreachableEndpoint(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	cases := map[string]string{
		"closed":    closed.URL,
		"not found": strings.TrimSuffix(up.URL, "/v1"),
		// A listing in the body, so only the status check rejects it.
		"error": serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte(`{"data":[],"error":{"message":"upstream secret detail"}}`))
		}),
		"not json": serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("upstream secret detail")) }),
		"no data": serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"error":{"message":"upstream secret detail"}}`))
		}),
		"null data": serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"data":null}`)) }),
		"null":      serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`null`)) }),
		// Following it would reach the working endpoint and succeed.
		"redirect": serve(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, up.URL+"/models", http.StatusFound)
		}),
		"over 1 MiB": serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"data":[{"id":"gpt-4o"}` + strings.Repeat(" ", 1<<20) + `]}`))
		}),
	}
	for name, base := range cases {
		res := env.Do("POST", "/api/providers", map[string]string{"name": name, "baseUrl": base, "apiKey": key}, session)
		body := strings.ToLower(string(res.Body))
		if res.Status != http.StatusBadRequest || res.ErrorCode() != "provider_unreachable" ||
			strings.Contains(body, "upstream secret detail") || strings.Contains(body, "teapot") {
			t.Errorf("%s: got %d %s, want 400 provider_unreachable without upstream details", name, res.Status, res.Body)
		}
	}
	if n := countProviders(t, env); n != 0 {
		t.Fatalf("%d providers stored", n)
	}
}

func TestCreateRejectsUnknownModel(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")

	env.Do("POST", "/api/providers", map[string]any{"name": "OpenAI", "baseUrl": up.URL, "apiKey": key, "models": []string{"gpt-4o", "gpt-5"}}, session).
		Expect(http.StatusBadRequest, "unknown_model")
	if n := countProviders(t, env); n != 0 {
		t.Fatalf("%d providers stored", n)
	}
}

func TestCreateRejectsDuplicateModel(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")

	env.Do("POST", "/api/providers", map[string]any{"name": "OpenAI", "baseUrl": up.URL, "apiKey": key, "models": []string{"gpt-4o", "gpt-4o"}}, session).
		Expect(http.StatusBadRequest, "duplicate_model")
	if n := countProviders(t, env); n != 0 {
		t.Fatalf("%d providers stored", n)
	}
}

func TestCreateRejectsTooManyModels(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")
	var models []string
	for i := range 101 {
		models = append(models, fmt.Sprint("model-", i))
	}
	up := newUpstream(t, key, models...)

	env.Do("POST", "/api/providers", map[string]any{"name": "Many", "baseUrl": up.URL, "apiKey": key, "models": models}, session).
		Expect(http.StatusBadRequest, "too_many_models")
	if n := countProviders(t, env); n != 0 {
		t.Fatalf("%d providers stored", n)
	}
	env.Do("POST", "/api/providers", map[string]any{"name": "Many", "baseUrl": up.URL, "apiKey": key, "models": models[:100]}, session).
		Expect(http.StatusCreated, "")
}

func TestCreateRefusesInternalAddresses(t *testing.T) {
	env := apptest.New(t, func(c *app.Config) { c.AllowPrivateProviderURLs = false })
	session := env.SignUp("ada@example.com", "correct horse")
	var hit atomic.Bool
	internal := serve(t, func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
		w.Write([]byte(`{"data":[{"id":"leak"}]}`))
	})

	env.Do("POST", "/api/providers", map[string]string{"name": "OpenAI", "baseUrl": internal, "apiKey": key}, session).
		Expect(http.StatusBadRequest, "provider_unreachable")
	if hit.Load() || countProviders(t, env) != 0 {
		t.Fatal("a loopback base URL was called or stored")
	}

	// Link-local addresses, such as cloud metadata, stay refused when private ones are allowed, and before any dial.
	env = apptest.New(t)
	session = env.SignUp("ada@example.com", "correct horse")
	for _, base := range []string{"http://169.254.169.254", "http://[::ffff:169.254.169.254]"} {
		start := time.Now()
		env.Do("POST", "/api/providers", map[string]string{"name": base, "baseUrl": base, "apiKey": key}, session).
			Expect(http.StatusBadRequest, "provider_unreachable")
		if d := time.Since(start); d > time.Second {
			t.Errorf("%s answered after %v, want it refused at once", base, d)
		}
	}
}

func TestCreateRejectsTakenName(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	up := newUpstream(t, key, "gpt-4o")

	create(t, env, ada, "OpenAI", up.URL, key)
	env.Do("POST", "/api/providers", map[string]string{"name": "OpenAI ", "baseUrl": up.URL, "apiKey": key}, ada).
		Expect(http.StatusConflict, "name_taken")
	create(t, env, bob, "OpenAI", up.URL, key)
}

func TestCreateNeedsSession(t *testing.T) {
	env := apptest.New(t)
	up := newUpstream(t, key, "gpt-4o")

	env.Do("POST", "/api/providers", map[string]string{"name": "OpenAI", "baseUrl": up.URL, "apiKey": key}).
		Expect(http.StatusUnauthorized, "unauthorized")
	if up.lastAuth() != "" || countProviders(t, env) != 0 {
		t.Fatal("request without a session reached the endpoint or stored a provider")
	}
}
