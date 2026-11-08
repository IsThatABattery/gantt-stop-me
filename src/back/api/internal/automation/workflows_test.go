package automation_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"gantt-stop-me/api/internal/automation"
)

// exampleWorkflows is this repository's own workflows file, verbatim from AUTOMATION.md.
const exampleWorkflows = `workflows:
  backend:
    description: Go API, endpoints, database
    context: src/back/api
    target: build              # the stage that has the source and module cache
    env:
      TEST_DATABASE_URL: postgres://gantt:gantt@test-db:5432/gantt?sslmode=disable
    cache: [/root/.cache/go-build]
    services:
      test-db:
        image: postgres:18
        env: { POSTGRES_USER: gantt, POSTGRES_PASSWORD: gantt, POSTGRES_DB: gantt }
        health: pg_isready -U gantt -d gantt
    timeout: 20m

  frontend:
    description: React UI seen in a browser, against the real API
    context: src/front
    port: 80                   # the app, at http://app:80
    services:
      db:
        image: postgres:18
        env: { POSTGRES_USER: gantt, POSTGRES_PASSWORD: gantt, POSTGRES_DB: gantt }
        health: pg_isready -U gantt -d gantt
      api:                     # nginx in the image proxies /api to http://api:8080
        context: src/back/api
        env:
          DATABASE_URL: postgres://gantt:gantt@db:5432/gantt?sslmode=disable
          PUBLIC_URL: http://app
          ENCRYPTION_KEY: rWhMzl7tTjnG0yUf8pzqa3gBBWudv5DWpXxrF6sgzB0=   # the public dev key
        port: 8080
    timeout: 30m
`

// file returns a workflows file with one workflow, ci, described "CI", with fields under it from line 4 on.
func file(fields string) string {
	return "workflows:\n  ci:\n    description: CI\n    " + strings.ReplaceAll(fields, "\n", "\n    ") + "\n"
}

// one parses in, which must hold exactly one workflow.
func one(t *testing.T, in string) automation.Workflow {
	t.Helper()
	ws, err := automation.ParseWorkflows([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 1 {
		t.Fatalf("%d workflows, want 1", len(ws))
	}
	return ws[0]
}

// specJSON checks that v encodes to the JSON want, which may be indented.
func specJSON(t *testing.T, v any, want string) {
	t.Helper()
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var w bytes.Buffer
	if err := json.Compact(&w, []byte(want)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, w.Bytes()) {
		t.Fatalf("JSON =\n%s\nwant\n%s", got, w.Bytes())
	}
}

func TestParseWorkflowsExample(t *testing.T) {
	ws, err := automation.ParseWorkflows([]byte(exampleWorkflows))
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 {
		t.Fatalf("%d workflows, want backend and frontend", len(ws))
	}
	backend, frontend := ws[0], ws[1]
	if backend.Description != "Go API, endpoints, database" || backend.Timeout != 20*time.Minute {
		t.Fatalf("backend description %q, timeout %v", backend.Description, backend.Timeout)
	}
	if frontend.Description != "React UI seen in a browser, against the real API" || frontend.Timeout != 30*time.Minute {
		t.Fatalf("frontend description %q, timeout %v", frontend.Description, frontend.Timeout)
	}
	db := `{"POSTGRES_DB": "gantt", "POSTGRES_PASSWORD": "gantt", "POSTGRES_USER": "gantt"}`
	specJSON(t, backend, `{
		"workflow": "backend",
		"context": "src/back/api", "dockerfile": "Dockerfile", "target": "build",
		"env": {"TEST_DATABASE_URL": "postgres://gantt:gantt@test-db:5432/gantt?sslmode=disable"},
		"cache": ["/root/.cache/go-build"],
		"port": null,
		"services": [{"name": "test-db", "image": "postgres:18", "env": `+db+`, "health": "pg_isready -U gantt -d gantt", "port": null}]
	}`)
	specJSON(t, frontend, `{
		"workflow": "frontend",
		"context": "src/front", "dockerfile": "Dockerfile", "target": "",
		"env": {},
		"cache": [],
		"port": 80,
		"services": [
			{"name": "db", "image": "postgres:18", "env": `+db+`, "health": "pg_isready -U gantt -d gantt", "port": null},
			{"name": "api", "context": "src/back/api", "dockerfile": "Dockerfile", "env": {
				"DATABASE_URL": "postgres://gantt:gantt@db:5432/gantt?sslmode=disable",
				"ENCRYPTION_KEY": "rWhMzl7tTjnG0yUf8pzqa3gBBWudv5DWpXxrF6sgzB0=",
				"PUBLIC_URL": "http://app"
			}, "port": 8080}
		]
	}`)
}

func TestDefaultWorkflows(t *testing.T) {
	ws := automation.DefaultWorkflows()
	if len(ws) != 1 || ws[0].Timeout != 30*time.Minute {
		t.Fatalf("DefaultWorkflows = %+v, want one with a 30m timeout", ws)
	}
	specJSON(t, ws[0], `{"workflow": "default", "context": ".", "dockerfile": "Dockerfile", "target": "",
		"env": {}, "cache": [], "port": null, "services": []}`)
}

// A file's workflow with only a description gets the same defaults as the default workflow, and a sidecar with no
// image is built with them.
func TestParseWorkflowsDefaults(t *testing.T) {
	w := one(t, file("services:\n  api:\n  web: {target: serve}"))
	if w.Timeout != 30*time.Minute {
		t.Fatalf("timeout %v, want 30m", w.Timeout)
	}
	specJSON(t, w, `{"workflow": "ci", "context": ".", "dockerfile": "Dockerfile", "target": "",
		"env": {}, "cache": [], "port": null, "services": [
			{"name": "api", "context": ".", "dockerfile": "Dockerfile", "env": {}, "port": null},
			{"name": "web", "context": ".", "dockerfile": "Dockerfile", "target": "serve", "env": {}, "port": null}
		]}`)
}

// port returns *p, or nil.
func port(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// names returns the name of each of xs.
func names[T any](xs []T, name func(T) string) []string {
	var ns []string
	for _, x := range xs {
		ns = append(ns, name(x))
	}
	return ns
}

func TestParseWorkflowsAccepts(t *testing.T) {
	sidecars := func(w automation.Workflow) any {
		return names(w.Services, func(s automation.Service) string { return s.Name })
	}
	for _, tc := range []struct {
		name string
		in   string
		got  func(automation.Workflow) any
		want any
	}{
		{"description trimmed", "workflows:\n  ci:\n    description: |\n      Go API\n", func(w automation.Workflow) any { return w.Description }, "Go API"},
		{"context inside", file("context: ./a/../src"), func(w automation.Workflow) any { return w.Context }, "./a/../src"},
		{"context starting with dots", file("context: ..src"), func(w automation.Workflow) any { return w.Context }, "..src"},
		{"dockerfile relative to context", file("dockerfile: ../Dockerfile\ncontext: src"),
			func(w automation.Workflow) any { return w.Context + " " + w.Dockerfile }, "src ../Dockerfile"},
		{"empty dockerfile is the default", file("dockerfile: ''"), func(w automation.Workflow) any { return w.Dockerfile }, "Dockerfile"},
		{"target", file("target: build"), func(w automation.Workflow) any { return w.Target }, "build"},
		{"env scalars as text", file("env:\n  A: 5432\n  B: true\n  C:\n  D: 'x y'"),
			func(w automation.Workflow) any { return w.Env }, map[string]string{"A": "5432", "B": "true", "C": "", "D": "x y"}},
		{"null env", file("env:"), func(w automation.Workflow) any { return w.Env }, map[string]string{}},
		{"four cache paths", file("cache: [/a, /b, /c/d, /e]"), func(w automation.Workflow) any { return w.Cache }, []string{"/a", "/b", "/c/d", "/e"}},
		{"null cache", file("cache:"), func(w automation.Workflow) any { return w.Cache }, []string{}},
		{"lowest port", file("port: 1"), func(w automation.Workflow) any { return port(w.Port) }, 1},
		{"highest port", file("port: 65535"), func(w automation.Workflow) any { return port(w.Port) }, 65535},
		{"null port", file("port:"), func(w automation.Workflow) any { return port(w.Port) }, nil},
		{"longest timeout", file("timeout: 2h"), func(w automation.Workflow) any { return w.Timeout }, 2 * time.Hour},
		{"short timeout", file("timeout: 90s"), func(w automation.Workflow) any { return w.Timeout }, 90 * time.Second},
		{"null timeout", file("timeout:"), func(w automation.Workflow) any { return w.Timeout }, 30 * time.Minute},
		{"sidecars in file order", file("services:\n  zeta: {image: z}\n  alpha: {image: a}\n  mid: {image: m}"),
			sidecars, []string{"zeta", "alpha", "mid"}},
		{"eight sidecars", file("services: {s1: {}, s2: {}, s3: {}, s4: {}, s5: {}, s6: {}, s7: {}, s8: {}}"),
			sidecars, []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8"}},
		{"null services", file("services:"), func(w automation.Workflow) any { return w.Services }, []automation.Service{}},
		{"built sidecar fields", file("services:\n  api: {context: src, dockerfile: ../Dockerfile.api, target: run}"),
			func(w automation.Workflow) any { return w.Services[0] },
			automation.Service{Name: "api", Context: "src", Dockerfile: "../Dockerfile.api", Target: "run", Env: map[string]string{}}},
		{"sidecar health and port", file("services:\n  db: {image: postgres:18, health: pg_isready, port: 5432, env: {A: 1}}"),
			func(w automation.Workflow) any {
				return []any{w.Services[0].Image, w.Services[0].Health, port(w.Services[0].Port), w.Services[0].Env}
			},
			[]any{"postgres:18", "pg_isready", 5432, map[string]string{"A": "1"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.got(one(t, tc.in)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestParseWorkflowsKeepsWorkflowOrder(t *testing.T) {
	ws, err := automation.ParseWorkflows([]byte("workflows:\n  zeta: {description: Z}\n  alpha: {description: A}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := names(ws, func(w automation.Workflow) string { return w.Name }); !reflect.DeepEqual(got, []string{"zeta", "alpha"}) {
		t.Fatalf("workflows %v, want zeta then alpha", got)
	}
}

func TestParseWorkflowsNames(t *testing.T) {
	for name, ok := range map[string]bool{
		"ab": true, "a-1": true, "test-db": true, strings.Repeat("a", 32): true,
		"a": false, "Backend": false, "1ab": false, "a_b": false, "-ab": false, "a.b": false, strings.Repeat("a", 33): false,
	} {
		t.Run(name, func(t *testing.T) {
			for _, in := range []string{
				fmt.Sprintf("workflows:\n  %s: {description: X}\n", name),
				file(fmt.Sprintf("services: {%s: {image: x}}", name)),
			} {
				_, err := automation.ParseWorkflows([]byte(in))
				if ok != (err == nil) {
					t.Fatalf("%q: err = %v, want ok %v", in, err, ok)
				}
				if !ok && !strings.Contains(err.Error(), "must be 2 to 32 lowercase letters, digits or -, starting with a letter") {
					t.Fatalf("%q: err = %v", in, err)
				}
			}
		})
	}
}

func TestParseWorkflowsRejects(t *testing.T) {
	const (
		outside = "must be a relative path inside the repository"
		badPort = "must be a port from 1 to 65535"
		badPath = "must be a clean absolute path other than /"
		badTime = "must be a duration above 0 and at most 2h, such as 20m"
	)
	for _, tc := range []struct {
		name string
		in   string
		want string // the error, or a part of a yaml error
	}{
		{"not yaml", "workflows: [\n", "did not find expected"},
		{"empty file", "", "no documents"},
		{"two documents", "workflows: {}\n---\nworkflows: {}\n", "expected single document"},
		{"empty document", "---\n", "workflows: needs at least one workflow"},
		{"top level a list", "- ci\n", "the file: must be a mapping (line 1)"},
		{"other top-level key", file("port: 80") + "version: 1\n", "version: is not a known field (line 5)"},
		{"no workflows", "{}\n", "workflows: needs at least one workflow"},
		{"empty workflows", "workflows: {}\n", "workflows: needs at least one workflow"},
		{"null workflows", "workflows:\n", "workflows: needs at least one workflow"},
		{"workflows a list", "workflows: [ci]\n", "workflows: must be a mapping (line 1)"},
		{"repeated top-level key", "workflows: {}\nworkflows: {}\n", "workflows: appears twice (line 2)"},
		{"repeated workflow", "workflows:\n  ci: {description: A}\n  ci: {description: B}\n", "workflows.ci: appears twice (line 3)"},
		{"repeated field", file("description: again"), "workflows.ci.description: appears twice (line 4)"},
		{"unknown field", file("colour: red"), "workflows.ci.colour: is not a known field (line 4)"},
		{"merge key", "workflows:\n  ci:\n    <<: {description: CI}\n", "workflows.ci.<<: is not a known field (line 3)"},
		{"alias", "workflows:\n  ci:\n    description: CI\n    env: &e {A: b}\n  cd:\n    description: CD\n    env: *e\n",
			"workflows.cd.env: is an alias, which this file cannot use (line 7)"},
		{"key not a string", file("env: {[a]: b}"), "workflows.ci.env: has a key that is not a string (line 4)"},
		{"workflow not a mapping", "workflows:\n  ci: hello\n", "workflows.ci: must be a mapping (line 2)"},
		{"no description", "workflows:\n  ci: {context: .}\n", "workflows.ci.description: is required (line 2)"},
		{"blank description", "workflows:\n  ci: {description: '  '}\n", "workflows.ci.description: is required (line 2)"},
		{"description a mapping", "workflows:\n  ci:\n    description: {a: b}\n", "workflows.ci.description: must be a string (line 3)"},
		{"absolute context", file("context: /src"), "workflows.ci.context: " + outside + " (line 4)"},
		{"context above the repository", file("context: .."), "workflows.ci.context: " + outside + " (line 4)"},
		{"context climbing out", file("context: src/../../x"), "workflows.ci.context: " + outside + " (line 4)"},
		{"absolute dockerfile", file("context: src\ndockerfile: /Dockerfile"), "workflows.ci.dockerfile: " + outside + " (line 5)"},
		{"dockerfile above the repository", file("dockerfile: ../Dockerfile"), "workflows.ci.dockerfile: " + outside + " (line 4)"},
		{"dockerfile climbing out of context", file("dockerfile: ../../Dockerfile\ncontext: src"), "workflows.ci.dockerfile: " + outside + " (line 4)"},
		{"env a list", file("env: [A]"), "workflows.ci.env: must be a mapping (line 4)"},
		{"env value a mapping", file("env:\n  A: {b: c}"), "workflows.ci.env.A: must be a string (line 5)"},
		{"env value a list", file("env: {A: [1]}"), "workflows.ci.env.A: must be a string (line 4)"},
		{"repeated env", file("env:\n  A: x\n  A: y"), "workflows.ci.env.A: appears twice (line 6)"},
		{"cache a string", file("cache: /a"), "workflows.ci.cache: must be a list (line 4)"},
		{"relative cache", file("cache: [a/b]"), "workflows.ci.cache[0]: " + badPath + " (line 4)"},
		{"unclean cache", file("cache: [/a, /b/../c]"), "workflows.ci.cache[1]: " + badPath + " (line 4)"},
		{"cache with trailing slash", file("cache: [/a/]"), "workflows.ci.cache[0]: " + badPath + " (line 4)"},
		{"cache of /", file("cache: [/]"), "workflows.ci.cache[0]: " + badPath + " (line 4)"},
		{"cache a mapping", file("cache: [{a: b}]"), "workflows.ci.cache[0]: must be a string (line 4)"},
		{"repeated cache", file("cache:\n  - /a\n  - /a"), "workflows.ci.cache[1]: repeats /a (line 6)"},
		{"five caches", file("cache: [/a, /b, /c, /d, /e]"), "workflows.ci.cache: has 5 paths, more than 4 (line 4)"},
		{"port 0", file("port: 0"), "workflows.ci.port: " + badPort + " (line 4)"},
		{"port 65536", file("port: 65536"), "workflows.ci.port: " + badPort + " (line 4)"},
		{"negative port", file("port: -1"), "workflows.ci.port: " + badPort + " (line 4)"},
		{"port a string", file("port: '80'"), "workflows.ci.port: " + badPort + " (line 4)"},
		{"port a word", file("port: http"), "workflows.ci.port: " + badPort + " (line 4)"},
		{"port a fraction", file("port: 80.5"), "workflows.ci.port: " + badPort + " (line 4)"},
		{"port a list", file("port: [80]"), "workflows.ci.port: " + badPort + " (line 4)"},
		{"zero timeout", file("timeout: 0s"), "workflows.ci.timeout: " + badTime + " (line 4)"},
		{"negative timeout", file("timeout: -1m"), "workflows.ci.timeout: " + badTime + " (line 4)"},
		{"timeout over 2h", file("timeout: 2h0m1s"), "workflows.ci.timeout: " + badTime + " (line 4)"},
		{"timeout without unit", file("timeout: 20"), "workflows.ci.timeout: " + badTime + " (line 4)"},
		{"timeout a word", file("timeout: soon"), "workflows.ci.timeout: " + badTime + " (line 4)"},
		{"timeout a list", file("timeout: [1m]"), "workflows.ci.timeout: must be a string (line 4)"},
		{"services a list", file("services: [db]"), "workflows.ci.services: must be a mapping (line 4)"},
		{"nine sidecars", file("services: {s1: {}, s2: {}, s3: {}, s4: {}, s5: {}, s6: {}, s7: {}, s8: {}, s9: {}}"),
			"workflows.ci.services: has 9 sidecars, more than 8 (line 4)"},
		{"sidecar called app", file("services: {app: {image: x}}"), "workflows.ci.services.app: is a reserved name (line 4)"},
		{"sidecar called browser", file("services: {browser: {image: x}}"), "workflows.ci.services.browser: is a reserved name (line 4)"},
		{"repeated sidecar", file("services:\n  db: {image: x}\n  db: {image: y}"), "workflows.ci.services.db: appears twice (line 6)"},
		{"sidecar not a mapping", file("services: {db: [x]}"), "workflows.ci.services.db: must be a mapping (line 4)"},
		{"unknown sidecar field", file("services:\n  db:\n    image: x\n    volumes: [/a]"), "workflows.ci.services.db.volumes: is not a known field (line 7)"},
		{"image with context", file("services:\n  db:\n    image: x\n    context: src"), "workflows.ci.services.db.context: cannot be set with image (line 7)"},
		{"image with dockerfile", file("services:\n  db:\n    dockerfile: D\n    image: x"), "workflows.ci.services.db.dockerfile: cannot be set with image (line 6)"},
		{"image with target", file("services:\n  db:\n    image: x\n    target: run"), "workflows.ci.services.db.target: cannot be set with image (line 7)"},
		{"image like a flag", file("services:\n  db:\n    image: --privileged"), "workflows.ci.services.db.image: must be an image reference (line 6)"},
		{"image a mapping", file("services:\n  db:\n    image: {a: b}"), "workflows.ci.services.db.image: must be a string (line 6)"},
		{"sidecar context outside", file("services:\n  api:\n    context: ../x"), "workflows.ci.services.api.context: " + outside + " (line 6)"},
		{"sidecar dockerfile outside", file("services:\n  api:\n    context: src\n    dockerfile: ../../D"), "workflows.ci.services.api.dockerfile: " + outside + " (line 7)"},
		{"sidecar env value a list", file("services:\n  db:\n    env: {A: [1]}"), "workflows.ci.services.db.env.A: must be a string (line 6)"},
		{"blank health", file("services:\n  db:\n    image: x\n    health: '  '"), "workflows.ci.services.db.health: must be a command (line 7)"},
		{"empty health", file("services:\n  db:\n    image: x\n    health:"), "workflows.ci.services.db.health: must be a command (line 7)"},
		{"sidecar port", "workflows:\n  frontend:\n    description: UI\n    services:\n      api:\n        port: 0\n",
			"workflows.frontend.services.api.port: " + badPort + " (line 6)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, err := automation.ParseWorkflows([]byte(tc.in))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseWorkflows = %v, %v; want error %q", ws, err, tc.want)
			}
		})
	}
}
