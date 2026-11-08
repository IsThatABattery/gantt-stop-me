package runner

import (
	"context"
	"net/http"

	"gantt-stop-me/api/internal/httpx"
)

// deleteExecution is DELETE /executions/{id}: stop the execution's requests, then remove its containers and network.
// Images and cache volumes stay. It works on executions the runner has forgotten, so it is safe to repeat.
func (s *Server) deleteExecution(w http.ResponseWriter, r *http.Request) {
	id, ok := execID(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	e := s.execs[id]
	delete(s.execs, id)
	s.mu.Unlock()
	if e != nil {
		e.stop()
		e.running.Wait() // so nothing it was starting appears after the removal
	}
	if err := s.remove(r.Context(), id); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteAll is DELETE /executions: the same for every execution with this runner's prefix.
func (s *Server) deleteAll(w http.ResponseWriter, r *http.Request) {
	if err := s.DeleteAll(r.Context()); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteAll stops every execution's requests and removes every container and network labelled with the prefix,
// including those of an earlier runner. main calls it on start.
func (s *Server) DeleteAll(ctx context.Context) error {
	s.mu.Lock()
	execs := s.execs
	s.execs = nil
	s.mu.Unlock()
	for _, e := range execs {
		e.stop()
	}
	for _, e := range execs {
		e.running.Wait()
	}
	return s.remove(ctx, 0)
}
