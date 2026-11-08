package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gantt-stop-me/api/internal/httpx"
)

// prepare is POST /executions/{id}: read the source, build the images, create the network,
// and start the sidecars and then the app. It answers once they are ready, with the log.
// What it starts stays, even on failure, until DELETE removes it.
func (s *Server) prepare(w http.ResponseWriter, r *http.Request) {
	id, ok := execID(w, r)
	if !ok {
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "The body must be multipart/form-data.")
		return
	}
	// The spec comes first, so it is checked before the tarball streams in.
	part, err := mr.NextPart()
	if err != nil || part.FormName() != "spec" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "The first part must be spec.")
		return
	}
	var spec Spec
	if err := json.NewDecoder(io.LimitReader(part, 1<<20)).Decode(&spec); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_spec", "The spec must be valid JSON.")
		return
	}
	if msg := spec.validate(); msg != "" {
		httpx.Error(w, http.StatusBadRequest, "bad_spec", msg)
		return
	}
	source, err := mr.NextPart()
	if err != nil || source.FormName() != "source" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "The second part must be source.")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	e := &execution{id: id, spec: spec}
	e.ctx, e.stop = context.WithCancel(context.Background())
	defer context.AfterFunc(e.ctx, cancel)()
	s.mu.Lock()
	if s.execs[id] != nil {
		s.mu.Unlock()
		httpx.Error(w, http.StatusConflict, "already_prepared", "This execution is already prepared. DELETE it first.")
		return
	}
	if s.execs == nil {
		s.execs = map[int64]*execution{}
	}
	s.execs[id] = e
	e.running.Add(1)
	s.mu.Unlock()
	defer e.running.Done()

	var log tail
	dir, err := os.MkdirTemp(s.Work, fmt.Sprintf("x%d-", id))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer os.RemoveAll(dir) // the images hold everything the containers need
	if err := extract(source, dir); err != nil {
		fmt.Fprintln(&log, err)
		fail(w, "build", &log)
		return
	}
	if err := s.build(ctx, &log, dir, spec); err != nil {
		fmt.Fprintln(&log, err)
		fail(w, "build", &log)
		return
	}
	if err := s.start(ctx, &log, e); err != nil {
		fmt.Fprintln(&log, err)
		fail(w, "start", &log)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"log": log.String()})
}

// step runs one docker command of a preparation, writing it and its output to log.
func step(ctx context.Context, log io.Writer, args ...string) error {
	fmt.Fprintf(log, "$ docker %s\n", strings.Join(args, " "))
	return docker(ctx, log, args...)
}

// build builds the test image and each sidecar that has a build, then pulls the sidecar images.
// It passes no --allow entitlements, secrets or SSH agent.
func (s *Server) build(ctx context.Context, log io.Writer, dir string, spec Spec) error {
	// Join cleans ".." away before docker sees the paths, so they only descend, and extract made sure every
	// symlink they pass through resolves inside dir.
	image := func(tag, context, dockerfile, target string) error {
		args := []string{"build", "--tag=" + tag, "--file=" + filepath.Join(dir, context, dockerfile)}
		if target != "" {
			args = append(args, "--target="+target)
		}
		return step(ctx, log, append(args, filepath.Join(dir, context))...)
	}
	if err := image(s.testImage(spec), spec.Context, spec.Dockerfile, spec.Target); err != nil {
		return err
	}
	for _, svc := range spec.Services {
		if svc.Image == "" {
			if err := image(s.serviceImage(spec, svc), svc.Context, svc.Dockerfile, svc.Target); err != nil {
				return err
			}
		}
	}
	for _, svc := range spec.Services {
		if svc.Image != "" {
			if err := step(ctx, log, "pull", svc.Image); err != nil {
				return err
			}
		}
	}
	return nil
}

// start creates e's network, joins the runner to it, then starts each sidecar in order and the app,
// each ready before the next starts.
func (s *Server) start(ctx context.Context, log io.Writer, e *execution) error {
	id, spec := e.id, e.spec
	network := s.network(id)
	if err := step(ctx, log, "network", "create", "--internal",
		"--label=gantt.prefix="+s.Prefix, "--label=gantt.execution="+strconv.FormatInt(id, 10), network); err != nil {
		return err
	}
	if err := step(ctx, log, "network", "connect", network, s.Self); err != nil {
		return err
	}
	for _, svc := range spec.Services {
		name := s.container(id, svc.Name)
		addr, err := s.launch(ctx, log, e, svc.Name, s.sidecarRun(id, spec, svc))
		if err != nil {
			return err
		}
		check := func(context.Context) bool { return true } // running counts as healthy
		switch {
		case svc.Health != "":
			check = func(ctx context.Context) bool {
				return docker(ctx, io.Discard, "exec", name, "sh", "-c", svc.Health) == nil
			}
		case svc.Port != nil:
			check = accepts(joinPort(addr, *svc.Port))
		}
		if !ready(ctx, check) {
			s.logs(ctx, log, name)
			return fmt.Errorf("sidecar %s was not healthy within %s", svc.Name, limit)
		}
	}
	if spec.Port != nil {
		addr, err := s.launch(ctx, log, e, "app", s.appRun(id, spec))
		if err != nil {
			return err
		}
		if !ready(ctx, accepts(joinPort(addr, *spec.Port))) {
			s.logs(ctx, log, s.container(id, "app"))
			return fmt.Errorf("app did not accept connections on port %d within %s", *spec.Port, limit)
		}
	}
	return nil
}

// launch runs a long-lived container of e with hostname host and records its address.
func (s *Server) launch(ctx context.Context, log io.Writer, e *execution, host string, run []string) (string, error) {
	if err := step(ctx, log, run...); err != nil {
		return "", err
	}
	addr, err := s.address(ctx, e.id, s.container(e.id, host))
	if err != nil {
		return "", err
	}
	s.setAddr(e, host, addr)
	return addr, nil
}

// logs appends a container's own output to log, to show why it did not become ready.
func (s *Server) logs(ctx context.Context, log io.Writer, container string) {
	fmt.Fprintf(log, "$ docker logs %s\n", container)
	docker(ctx, log, "logs", "--tail=200", container)
}

// joinPort is addr:port.
func joinPort(addr string, port int) string {
	return net.JoinHostPort(addr, strconv.Itoa(port))
}
