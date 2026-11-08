package runner

import (
	"cmp"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gantt-stop-me/api/internal/httpx"
)

// HTTPRequest is one HTTP request to make on an execution's network.
type HTTPRequest struct {
	Method  string            `json:"method"` // "" means GET
	URL     string            `json:"url"`    // http, with host app or a sidecar's name
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// HTTPResponse is the answer to an HTTPRequest, with its body cut at 64 KB.
type HTTPResponse struct {
	Status  int         `json:"status"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
}

// request is POST /executions/{id}/http: make one HTTP request to the app or a sidecar.
// It dials the container's address, not its name, which could resolve on another of the runner's networks,
// and keeps the URL's host as the Host header. Redirects are not followed.
func (s *Server) request(w http.ResponseWriter, r *http.Request, e *execution) {
	var in HTTPRequest
	if !httpx.Decode(w, r, &in) {
		return
	}
	u, err := url.Parse(in.URL)
	if err != nil || u.Scheme != "http" {
		httpx.Error(w, http.StatusBadRequest, "bad_url", "The URL must be http.")
		return
	}
	addr := ""
	if host := u.Hostname(); host != "browser" {
		addr = s.addr(e, host)
	}
	if addr == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_host", "The URL's host must be app or a sidecar's name.")
		return
	}
	addr = net.JoinHostPort(addr, cmp.Or(u.Port(), "80"))
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, cmp.Or(in.Method, http.MethodGet), in.URL, strings.NewReader(in.Body))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	for k, v := range in.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	res, err := client.Do(req)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "unreachable", err.Error())
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, keep))
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "unreachable", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, HTTPResponse{Status: res.StatusCode, Headers: res.Header, Body: string(body)})
}
