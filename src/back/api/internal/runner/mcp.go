package runner

import (
	"net"
	"net/http"

	"gantt-stop-me/api/internal/httpx"
)

// mcpHeaders are the request headers forwarded to the browser. The runner's token is not one of them.
var mcpHeaders = []string{"Content-Type", "Accept", "Mcp-Session-Id", "Mcp-Protocol-Version"}

// mcpClient talks to browsers. No keep-alives, since a browser's address is reused once its network is removed.
var mcpClient = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

// mcp is /executions/{id}/mcp: forward one MCP request to the browser's streamable HTTP endpoint, and stream back
// its answer, which may be text/event-stream, with its status, Content-Type and Mcp-Session-Id.
func (s *Server) mcp(w http.ResponseWriter, r *http.Request, e *execution) {
	addr := s.addr(e, "browser")
	if addr == "" {
		httpx.Error(w, http.StatusConflict, "no_browser", "Start the browser first.")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, "http://"+net.JoinHostPort(addr, browserPort)+"/mcp", r.Body)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	req.Host = browserHost
	for _, h := range mcpHeaders {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	res, err := mcpClient.Do(req)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "unreachable", err.Error())
		return
	}
	defer res.Body.Close()
	for _, h := range []string{"Content-Type", "Mcp-Session-Id"} {
		if v := res.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(res.StatusCode)
	flush := http.NewResponseController(w)
	buf := make([]byte, 32<<10)
	for {
		n, err := res.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			flush.Flush()
		}
		if err != nil {
			return
		}
	}
}
