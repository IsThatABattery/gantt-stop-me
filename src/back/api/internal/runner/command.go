package runner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"gantt-stop-me/api/internal/httpx"
)

// command is POST /executions/{id}/commands: run sh -c with the command in a fresh container from the test image,
// and answer its exit code and the last 64 KB of its stdout and stderr together.
func (s *Server) command(w http.ResponseWriter, r *http.Request, e *execution) {
	var in struct {
		Command string `json:"command"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	// "_" cannot be in a sidecar name, so this never collides with one.
	name := fmt.Sprintf("%s_%d", s.network(e.id), e.cmds.Add(1))
	defer func() {
		// The container outlives a killed docker CLI, so remove it even when the request is gone.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Second)
		defer cancel()
		dockerOutput(ctx, "rm", "--force", "--volumes", name)
	}()
	var out tail
	err := docker(r.Context(), &out, s.commandRun(e.id, e.spec, name, in.Command)...)
	code := 0
	if exit := (*exec.ExitError)(nil); errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if r.Context().Err() != nil {
		httpx.Error(w, http.StatusConflict, "cancelled", "The execution was deleted while the command ran.")
		return
	}
	// shortcut: 125 is docker's own failure, such as a missing image, so it answers as a runner error;
	// a command that itself exits 125 is misread, give the test container an exit-code file if that bites.
	if code == 125 {
		httpx.Error(w, http.StatusInternalServerError, "docker", out.String())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"exitCode": code, "output": out.String()})
}
