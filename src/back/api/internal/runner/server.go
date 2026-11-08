package runner

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gantt-stop-me/api/internal/httpx"
)

// Server is the runner's HTTP API. It keeps each execution's state in memory, so a restart forgets them;
// main removes everything labelled with Prefix before serving.
type Server struct {
	Token  string // RUNNER_TOKEN: every request but GET /health must carry it as a bearer token
	Prefix string // RUN_PREFIX, on every image, container, network and volume the runner makes
	Work   string // where source checkouts go: /work, a tmpfs, in the runner container
	Self   string // the runner's own container, joined to every execution network

	mu       sync.Mutex
	execs    map[int64]*execution
	deleting sync.Mutex // held by each DELETE, from stopping requests to the end of the removal
}

// Listen listens on port at each IPv4 address the runner has now. main calls it before the runner joins any
// execution network, so test containers, which share those networks, cannot reach the API, whatever token they hold.
func Listen(port string) ([]net.Listener, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	var ls []net.Listener
	for _, a := range addrs {
		if ip, ok := a.(*net.IPNet); ok && ip.IP.To4() != nil {
			l, err := net.Listen("tcp", net.JoinHostPort(ip.IP.String(), port))
			if err != nil {
				for _, l := range ls {
					l.Close()
				}
				return nil, err
			}
			ls = append(ls, l)
		}
	}
	return ls, nil
}

// execution is one prepared or preparing workflow execution.
type execution struct {
	id   int64
	spec Spec
	ctx  context.Context // ended by DELETE, which stops every request running on the execution
	stop context.CancelFunc

	running sync.WaitGroup // requests on the execution, which DELETE waits for before it removes containers
	addrs   map[string]string
	browser sync.Mutex   // held while the browser starts, so a repeat waits for the first
	cmds    atomic.Int64 // numbers command containers
}

// Handler serves the runner's endpoints.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /executions/{id}", s.prepare)
	mux.HandleFunc("POST /executions/{id}/commands", s.exec(s.command))
	mux.HandleFunc("POST /executions/{id}/http", s.exec(s.request))
	mux.HandleFunc("POST /executions/{id}/browser", s.exec(s.startBrowser))
	mux.HandleFunc("GET /executions/{id}/browser/videos/{name}", s.exec(s.video))
	mux.HandleFunc("/executions/{id}/mcp", s.exec(s.mcp)) // any method: MCP also GETs and DELETEs sessions
	mux.HandleFunc("DELETE /executions/{id}", s.deleteExecution)
	mux.HandleFunc("DELETE /executions", s.deleteAll)

	outer := http.NewServeMux()
	outer.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	outer.Handle("/", s.auth(mux))
	return outer
}

// auth answers 401 unless the request carries the runner's token, compared in constant time.
func (s *Server) auth(next http.Handler) http.Handler {
	want := []byte("Bearer " + s.Token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "A valid runner token is required.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// execID is the request's execution id. On a bad one it answers 404 and returns false.
func execID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such execution.")
		return 0, false
	}
	return id, true
}

// exec runs h on the execution the request names, or answers 404. The request counts as running on it,
// and its context also ends when DELETE removes the execution.
func (s *Server) exec(h func(http.ResponseWriter, *http.Request, *execution)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := execID(w, r)
		if !ok {
			return
		}
		s.mu.Lock()
		e := s.execs[id]
		if e != nil {
			e.running.Add(1)
		}
		s.mu.Unlock()
		if e == nil {
			httpx.Error(w, http.StatusNotFound, "not_found", "No such execution. Prepare it first.")
			return
		}
		defer e.running.Done()
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		defer context.AfterFunc(e.ctx, cancel)()
		h(w, r.WithContext(ctx), e)
	}
}

// addr is the address of e's container with hostname host, or "" when there is none.
func (s *Server) addr(e *execution, host string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return e.addrs[host]
}

// setAddr records the address of e's container with hostname host.
func (s *Server) setAddr(e *execution, host, addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.addrs == nil {
		e.addrs = map[string]string{}
	}
	e.addrs[host] = addr
}

// fail answers 422 {"error": stage, "log": log}: an execution's build or start failed.
func fail(w http.ResponseWriter, stage string, log *tail) {
	httpx.JSON(w, http.StatusUnprocessableEntity, Failure{Stage: stage, Log: log.String()})
}

// limit is how long a sidecar, the app or the browser has to become ready.
const limit = 2 * time.Minute

// ready tries check once a second until it succeeds, for up to limit.
// shortcut: a container that exits is noticed only at the limit, check its state when that wait hurts.
func ready(ctx context.Context, check func(context.Context) bool) bool {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	for !check(ctx) {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Second):
		}
	}
	return true
}

// accepts reports whether addr accepts a TCP connection.
func accepts(addr string) func(context.Context) bool {
	return func(ctx context.Context) bool {
		c, err := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "tcp", addr)
		if err == nil {
			c.Close()
		}
		return err == nil
	}
}

// keep is how much of a log or command output the runner returns: the last 64 KB.
const keep = 64 << 10

// tail keeps the last 64 KB written to it.
type tail struct{ b []byte }

func (t *tail) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > keep {
		t.b = t.b[len(t.b)-keep:]
	}
	return len(p), nil
}

func (t *tail) String() string { return string(t.b) }
