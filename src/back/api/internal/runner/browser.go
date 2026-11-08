package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	"gantt-stop-me/api/internal/httpx"
)

// policyPath is where the browser image's Chromium, a Chrome for Testing build, reads a managed policy.
const policyPath = "etc/opt/chrome_for_testing/policies/managed/gantt.json"

// startBrowser is POST /executions/{id}/browser: start the browser on the execution's network, and answer
// once its MCP endpoint accepts connections. A repeat finds it started and only waits for it again.
func (s *Server) startBrowser(w http.ResponseWriter, r *http.Request, e *execution) {
	if e.spec.Port == nil {
		httpx.Error(w, http.StatusConflict, "no_port", "The workflow has no port, so it gets no browser.")
		return
	}
	e.browser.Lock()
	defer e.browser.Unlock()
	var log tail
	addr := s.addr(e, "browser")
	if addr == "" {
		// A launch cut short, such as by a dropped request, can leave the container behind under its name.
		docker(r.Context(), io.Discard, "rm", "--force", s.container(e.id, "browser"))
		var err error
		if addr, err = s.launch(r.Context(), &log, e, "browser", s.browserRun(e.id)); err == nil {
			err = s.policy(r.Context(), &log, e)
		}
		if err != nil {
			s.setAddr(e, "browser", "") // so a repeat starts it over, policy and all
			fmt.Fprintln(&log, err)
			fail(w, "start", &log)
			return
		}
	}
	if !ready(r.Context(), accepts(net.JoinHostPort(addr, browserPort))) {
		s.logs(r.Context(), &log, s.container(e.id, "browser"))
		fmt.Fprintf(&log, "the browser did not accept connections within %s\n", limit)
		fail(w, "start", &log)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// policy copies into e's browser container a Chromium policy that lets the browser reach the app and the sidecars
// over http. Chromium preloads HSTS for whole TLDs, app among them, so it would turn http://app/ into https://app/,
// which nothing serves.
// shortcut: it relies on the pinned image starting Chromium at the first tool call, which comes after startBrowser
// answers; create the container, copy, then start it if an image starts Chromium at once.
func (s *Server) policy(ctx context.Context, log io.Writer, e *execution) error {
	hosts := []string{"app"}
	for _, svc := range e.spec.Services {
		hosts = append(hosts, svc.Name)
	}
	policy, _ := json.Marshal(map[string][]string{"HSTSPolicyBypassList": hosts}) // strings only, so it cannot fail
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: policyPath, Mode: 0o644, Size: int64(len(policy))}); err != nil {
		return err
	}
	tw.Write(policy)
	if err := tw.Close(); err != nil {
		return err
	}
	dst := s.container(e.id, "browser") + ":/"
	fmt.Fprintf(log, "$ docker cp - %s\n", dst)
	return dockerIn(ctx, &archive, log, "cp", "-", dst) // root-owned, so the browser cannot change it
}
