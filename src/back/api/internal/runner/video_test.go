package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestVideoRefusesBadNamesAndNoBrowser(t *testing.T) {
	s := &Server{}
	c := fakeExecution(t, s, map[string]string{})
	for _, name := range []string{"R1", "r1.webm", "r_1", strings.Repeat("a", 65)} {
		if _, err := c.Video(context.Background(), 1, name); !isRunnerError(err, 400, "bad_name") {
			t.Errorf("Video(%q) = %v, want 400 bad_name", name, err)
		}
	}
	if _, err := c.Video(context.Background(), 1, "r1"); !isRunnerError(err, 404, "not_found") {
		t.Errorf("Video() before the browser = %v, want 404 not_found", err)
	}
}

// isRunnerError reports whether err is the runner answering status with code.
func isRunnerError(err error, status int, code string) bool {
	e := (*Error)(nil)
	return errors.As(err, &e) && e.Status == status && e.Code == code
}
