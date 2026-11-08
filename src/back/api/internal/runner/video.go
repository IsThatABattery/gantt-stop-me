package runner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"gantt-stop-me/api/internal/httpx"
)

// browserOutput is the browser's output directory, on the tmpfs every container gets at /tmp. The browser lets a
// tool write only there and in its working directory.
const browserOutput = "/tmp/browser"

// VideoFile is where the browser saves recording name: the filename the API gives browser_start_video, and the file
// GET /executions/{id}/browser/videos/{name} answers. A relative filename would land in the browser's working
// directory, off the tmpfs.
func VideoFile(name string) string { return browserOutput + "/" + name + ".webm" }

// maxVideo is the most of one recording the runner answers and the client reads.
const maxVideo = 64 << 20

// videoName is a recording's name, which becomes a file name in the browser container.
var videoName = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// video is GET /executions/{id}/browser/videos/{name}: stream the browser's recording name, then delete it from the
// browser container. One over maxVideo answers 413 too_large, and is deleted too. docker cp cannot read a tmpfs,
// so the recording comes out through docker exec.
func (s *Server) video(w http.ResponseWriter, r *http.Request, e *execution) {
	name := r.PathValue("name")
	if !videoName.MatchString(name) {
		httpx.Error(w, http.StatusBadRequest, "bad_name", "The name must be 1 to 64 lowercase letters, digits or -.")
		return
	}
	if s.addr(e, "browser") == "" {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such recording.")
		return
	}
	browser, file := s.container(e.id, "browser"), VideoFile(name)
	defer func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Second)
		defer cancel()
		dockerOutput(ctx, "exec", browser, "rm", "-f", file)
	}()
	// Exit 3 is the script's own no such file, which neither docker exec nor stat exits with.
	out, err := dockerOutput(r.Context(), "exec", browser, "sh", "-c", `test -f "$1" || exit 3; stat -c %s "$1"`, "sh", file)
	if exit := (*exec.ExitError)(nil); errors.As(err, &exit) && exit.ExitCode() == 3 {
		httpx.Error(w, http.StatusNotFound, "not_found", "No such recording.")
		return
	}
	var size int64
	if err == nil {
		size, err = strconv.ParseInt(out, 10, 64)
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if size > maxVideo {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("The recording is over %d MiB.", maxVideo>>20))
		return
	}
	w.Header().Set("Content-Type", "video/webm")
	// It also caps the body, should the file have grown since. A failure cuts the body short, which the client sees.
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	cmd := exec.CommandContext(r.Context(), "docker", "exec", browser, "cat", file)
	cmd.Env, cmd.Stdout = dockerEnv(), w
	cmd.Run()
}
