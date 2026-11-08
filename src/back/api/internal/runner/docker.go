package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// browserImage is the Playwright MCP server the browser runs. Its entrypoint already passes
// --headless --browser chromium --no-sandbox; browserRun adds the rest.
const browserImage = "mcr.microsoft.com/playwright/mcp:v0.0.83"

// browserPort is where the browser serves MCP's streamable HTTP, at /mcp.
const browserPort = "8931"

// browserHost is the Host header every request to the browser carries, and the only one its host check allows.
// The runner dials the browser's address, so the header is set rather than taken from the URL.
const browserHost = "browser:" + browserPort

// dockerEnv is the docker CLI's whole environment: PATH, HOME and DOCKER_HOST, so nothing else the runner holds,
// such as RUNNER_TOKEN, reaches a build or a container.
func dockerEnv() []string {
	env := []string{} // not nil: a nil Env would hand the child the runner's whole environment
	for _, k := range []string{"PATH", "HOME", "DOCKER_HOST"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// docker runs the docker CLI with stdout and stderr going to out. Cancelling ctx kills it.
func docker(ctx context.Context, out io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = dockerEnv()
	cmd.Stdout, cmd.Stderr = out, out
	return cmd.Run()
}

// dockerOutput runs the docker CLI and returns its trimmed stdout. A failure's error carries stderr.
func dockerOutput(ctx context.Context, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = dockerEnv()
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// run is `docker run` for one container of execution id: the fixed run flags, which no spec can change,
// then extra, then "--" and the image with its arguments.
func (s *Server) run(id int64, extra []string, image string, args ...string) []string {
	a := []string{"run",
		"--network=" + s.network(id),
		"--security-opt=no-new-privileges",
		"--cap-drop=NET_RAW", "--cap-drop=MKNOD",
		"--memory=4g", "--cpus=2", "--pids-limit=1024",
		"--tmpfs=/tmp",
		"--label=gantt.prefix=" + s.Prefix,
		"--label=gantt.execution=" + strconv.FormatInt(id, 10),
	}
	a = append(a, extra...)
	return append(append(a, "--", image), args...)
}

// service is the flags of a long-running container the others reach by name: app, a sidecar or the browser.
func (s *Server) service(id int64, host string) []string {
	return []string{"--detach", "--name=" + s.container(id, host), "--hostname=" + host, "--network-alias=" + host}
}

// envFlags sets env in a container. Each is one --env=K=V argument, so no value can become a flag.
func envFlags(env map[string]string) []string {
	var a []string
	for k, v := range env {
		a = append(a, "--env="+k+"="+v)
	}
	return a
}

// testFlags are what every container from the test image gets: the workflow's env and its cache volumes.
func (s *Server) testFlags(spec Spec) []string {
	a := envFlags(spec.Env)
	for n, p := range spec.Cache {
		a = append(a, "--mount=type=volume,src="+s.cacheVolume(spec, n)+",dst="+p)
	}
	return a
}

// sidecarRun starts sidecar svc under its own name.
func (s *Server) sidecarRun(id int64, spec Spec, svc Service) []string {
	return s.run(id, append(s.service(id, svc.Name), envFlags(svc.Env)...), s.serviceImage(spec, svc))
}

// appRun starts the test image's default command as app.
func (s *Server) appRun(id int64, spec Spec) []string {
	return s.run(id, append(s.service(id, "app"), s.testFlags(spec)...), s.testImage(spec))
}

// commandRun runs command with sh -c in a fresh container from the test image, attached, named name.
func (s *Server) commandRun(id int64, spec Spec, name, command string) []string {
	return s.run(id, append([]string{"--name=" + name, "--entrypoint=sh"}, s.testFlags(spec)...), s.testImage(spec), "-c", command)
}

// browserRun starts the browser, isolated, serving MCP on every interface of its network.
func (s *Server) browserRun(id int64) []string {
	return s.run(id, s.service(id, "browser"), browserImage,
		"--isolated", "--host", "0.0.0.0", "--port", browserPort, "--allowed-hosts", browserHost)
}

// address is container's IPv4 address on execution id's network, where the runner dials it.
// Names are ambiguous to the runner, which sits on many networks at once.
func (s *Server) address(ctx context.Context, id int64, container string) (string, error) {
	addr, err := dockerOutput(ctx, "inspect", "--format",
		`{{(index .NetworkSettings.Networks "`+s.network(id)+`").IPAddress}}`, container)
	if err == nil && addr == "" {
		err = fmt.Errorf("%s has no address on %s", container, s.network(id))
	}
	return addr, err
}

// remove removes the containers and networks labelled with this runner's prefix, and with execution id when it
// is not 0. Images and cache volumes stay. It disconnects the runner from each network first.
func (s *Server) remove(ctx context.Context, id int64) error {
	filter := []string{"--filter", "label=gantt.prefix=" + s.Prefix}
	if id != 0 {
		filter = append(filter, "--filter", "label=gantt.execution="+strconv.FormatInt(id, 10))
	}
	ids, err := dockerOutput(ctx, append([]string{"ps", "--all", "--quiet"}, filter...)...)
	if err != nil {
		return err
	}
	if ids != "" {
		// -v takes the anonymous volumes images such as postgres declare, which nothing else would remove.
		if _, err := dockerOutput(ctx, append([]string{"rm", "--force", "--volumes"}, strings.Fields(ids)...)...); err != nil {
			return err
		}
	}
	nets, err := dockerOutput(ctx, append([]string{"network", "ls", "--quiet"}, filter...)...)
	if err != nil || nets == "" {
		return err
	}
	for _, n := range strings.Fields(nets) {
		dockerOutput(ctx, "network", "disconnect", "--force", n, s.Self) // fails when not connected, which is fine
	}
	_, err = dockerOutput(ctx, append([]string{"network", "rm"}, strings.Fields(nets)...)...)
	return err
}
