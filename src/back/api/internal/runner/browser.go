package runner

import (
	"fmt"
	"io"
	"net"
	"net/http"

	"gantt-stop-me/api/internal/httpx"
)

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
		if addr, err = s.launch(r.Context(), &log, e, "browser", s.browserRun(e.id)); err != nil {
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
