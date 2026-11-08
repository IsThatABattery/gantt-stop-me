package runner

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// hostileSpec passes validation but puts flag-like text everywhere the repository controls.
func hostileSpec() Spec {
	port, dbPort := 80, 5432
	return Spec{
		Project: 3, Workflow: "web", Context: ".", Dockerfile: "Dockerfile",
		Env:   map[string]string{"A": "--privileged", "--cap-add=ALL": "x", "B": "1 --volume=/:/host --network=host"},
		Cache: []string{"/root/.cache/go-build", "/var/cache/--privileged"},
		Port:  &port,
		Services: []Service{
			{Name: "db", Image: "postgres:18", Env: map[string]string{"P": "--device=/dev/sda"}, Health: "--privileged; pg_isready"},
			{Name: "api", Context: "src/back/api", Dockerfile: "Dockerfile", Target: "--privileged", Env: map[string]string{"-v": "/var/run/docker.sock:/var/run/docker.sock"}, Port: &dbPort},
		},
	}
}

// fixedFlags are the Run flags table for execution 7 of a runner with prefix gantt.
var fixedFlags = []string{
	"--network=gantt-x7",
	"--security-opt=no-new-privileges",
	"--cap-drop=NET_RAW", "--cap-drop=MKNOD",
	"--memory=4g", "--cpus=2", "--pids-limit=1024",
	"--tmpfs=/tmp:exec",
	"--label=gantt.prefix=gantt", "--label=gantt.execution=7",
}

// forbidden are flags no container may get, by prefix.
var forbidden = []string{
	"--privileged", "--cap-add", "--device", "--volume", "-v", "--volumes-from", "--mount=type=bind", "--mount=type=tmpfs",
	"--publish", "-p", "-P", "--network=host", "--net=", "--pid=", "--ipc", "--uts", "--userns", "--user", "-u",
	"--security-opt=seccomp", "--security-opt=apparmor", "--security-opt=label", "--runtime", "--cgroup-parent", "--group-add",
}

func TestRunFlags(t *testing.T) {
	s := &Server{Prefix: "gantt"}
	spec := hostileSpec()
	if msg := spec.validate(); msg != "" {
		t.Fatalf("hostileSpec is invalid: %s", msg)
	}
	mounts := []string{
		"--mount=type=volume,src=gantt-p3-web-cache0,dst=/root/.cache/go-build",
		"--mount=type=volume,src=gantt-p3-web-cache1,dst=/var/cache/--privileged",
	}
	envs := func(env map[string]string) []string {
		var a []string
		for k, v := range env {
			a = append(a, "--env="+k+"="+v)
		}
		return a
	}
	service := func(host string) []string {
		return []string{"--detach", "--name=gantt-x7-" + host, "--hostname=" + host, "--network-alias=" + host}
	}
	command := "--privileged; rm -rf /"
	kinds := []struct {
		name  string
		args  []string
		flags []string // everything before "--" but the fixed flags
		image string
		cmd   []string
	}{
		{"image sidecar", s.sidecarRun(7, spec, spec.Services[0]), append(service("db"), envs(spec.Services[0].Env)...), "postgres:18", nil},
		{"build sidecar", s.sidecarRun(7, spec, spec.Services[1]), append(service("api"), envs(spec.Services[1].Env)...), "gantt-p3-web_api", nil},
		{"app", s.appRun(7, spec), slices.Concat(service("app"), envs(spec.Env), mounts), "gantt-p3-web", nil},
		{"command", s.commandRun(7, spec, "gantt-x7_1", command),
			slices.Concat([]string{"--name=gantt-x7_1", "--entrypoint=sh"}, envs(spec.Env), mounts), "gantt-p3-web", []string{"-c", command}},
		{"browser", s.browserRun(7), service("browser"), browserImage,
			[]string{"--isolated", "--host", "0.0.0.0", "--port", "8931", "--allowed-hosts", "browser:8931"}},
	}
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			end := slices.Index(k.args, "--")
			if k.args[0] != "run" || end < 0 {
				t.Fatalf("args = %q, want run ... -- image", k.args)
			}
			flags := k.args[1:end]
			for _, f := range flags {
				for _, bad := range forbidden {
					if strings.HasPrefix(f, bad) {
						t.Errorf("flag %q is forbidden", f)
					}
				}
			}
			// Exactly the fixed flags plus the kind's own: nothing the spec says adds another.
			got, want := slices.Clone(flags), slices.Concat(fixedFlags, k.flags)
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("flags =\n%q\nwant\n%q", got, want)
			}
			if image, cmd := k.args[end+1], k.args[end+2:]; image != k.image || !slices.Equal(cmd, k.cmd) {
				t.Errorf("image and command = %q %q, want %q %q", image, cmd, k.image, k.cmd)
			}
		})
	}
}

func TestNames(t *testing.T) {
	s := &Server{Prefix: "gantt"}
	spec := Spec{Project: 12, Workflow: "web", Services: []Service{{Name: "api"}, {Name: "cache", Image: "redis:8"}}}
	for got, want := range map[string]string{
		s.testImage(spec):                      "gantt-p12-web",
		s.serviceImage(spec, spec.Services[0]): "gantt-p12-web_api",
		s.serviceImage(spec, spec.Services[1]): "redis:8",
		s.cacheVolume(spec, 0):                 "gantt-p12-web-cache0",
		s.network(5):                           "gantt-x5",
		s.container(5, "api"):                  "gantt-x5-api",
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	// Workflow web-api's test image must not be workflow web's api sidecar.
	if other := s.testImage(Spec{Project: 12, Workflow: "web-api"}); other == s.serviceImage(spec, spec.Services[0]) {
		t.Errorf("workflow web-api and web's sidecar api share the tag %q", other)
	}
}

func TestDockerEnv(t *testing.T) {
	t.Setenv("RUNNER_TOKEN", "secret")
	t.Setenv("DOCKER_HOST", "tcp://docker:2375")
	t.Setenv("HOME", "/root")
	env := dockerEnv()
	want := []string{"PATH=" + os.Getenv("PATH"), "HOME=/root", "DOCKER_HOST=tcp://docker:2375"}
	if !slices.Equal(env, want) {
		t.Errorf("dockerEnv() = %q, want %q", env, want)
	}

	t.Setenv("PATH", "")
	for _, k := range []string{"PATH", "HOME", "DOCKER_HOST"} {
		os.Unsetenv(k)
	}
	// An empty Env, not nil, which would pass the runner's whole environment on.
	if env := dockerEnv(); env == nil || len(env) != 0 {
		t.Errorf("dockerEnv() with nothing set = %#v, want empty and not nil", env)
	}
}
