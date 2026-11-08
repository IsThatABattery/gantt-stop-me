package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

// newRunner serves a runner whose checkouts go to a temporary directory, and returns a client for it.
func newRunner(t *testing.T) (*Server, *Client) {
	t.Helper()
	s := &Server{Token: "t", Prefix: "gantt-unit", Work: t.TempDir()}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return s, &Client{URL: srv.URL, Token: "t", HTTPClient: srv.Client()}
}

func TestPrepareRefusesABadSpec(t *testing.T) {
	_, c := newRunner(t)
	spec := hostileSpec()
	spec.Cache = []string{"/a,type=bind,src=/var/run/docker.sock"}
	_, err := c.Prepare(context.Background(), 1, spec, bytes.NewReader(tarball(t, file("r/Dockerfile", "FROM scratch"))))
	if e := (*Error)(nil); !errors.As(err, &e) || e.Status != 400 || e.Code != "bad_spec" {
		t.Fatalf("Prepare() = %v, want 400 bad_spec", err)
	}
}

func TestPrepareRefusesAHostileTarball(t *testing.T) {
	s, c := newRunner(t)
	source := tarball(t, file("r/Dockerfile", "FROM scratch"), symlink("r/key", "/proc/1/environ"))
	_, err := c.Prepare(context.Background(), 2, hostileSpec(), bytes.NewReader(source))
	if f := (*Failure)(nil); !errors.As(err, &f) || f.Stage != "build" || !strings.Contains(f.Log, "r/key: symlink to an absolute path") {
		t.Fatalf("Prepare() = %v, want a build failure naming r/key", err)
	}
	if left, _ := os.ReadDir(s.Work); len(left) != 0 {
		t.Errorf("the checkout stayed in the work directory: %v", left)
	}
	// It stays registered until DELETE, so a second prepare of the same id is refused.
	_, err = c.Prepare(context.Background(), 2, hostileSpec(), bytes.NewReader(source))
	if e := (*Error)(nil); !errors.As(err, &e) || e.Status != http.StatusConflict {
		t.Errorf("a second Prepare() = %v, want 409", err)
	}
}

func TestPrepareWantsTheSpecFirst(t *testing.T) {
	_, c := newRunner(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("source", "source.tar.gz")
	part.Write(tarball(t))
	part, _ = mw.CreateFormField("spec")
	part.Write([]byte(`{"project": 1, "workflow": "web", "context": ".", "dockerfile": "Dockerfile"}`))
	mw.Close()
	err := c.send(context.Background(), "POST", c.path(3, ""), mw.FormDataContentType(), &body, nil)
	if e := (*Error)(nil); !errors.As(err, &e) || e.Status != 400 || !strings.Contains(e.Message, "spec") {
		t.Errorf("source before spec = %v, want 400", err)
	}
}

func TestDeleteStopsAnUpload(t *testing.T) {
	s, c := newRunner(t)
	ctx := context.Background()
	source, stall := io.Pipe()
	defer stall.Close() // so a failing test does not leave the handler reading
	// A gzip header, then nothing more.
	go stall.Write(tarball(t, file("r/Dockerfile", "FROM scratch"))[:20])
	prepared := make(chan error, 1)
	go func() {
		_, err := c.Prepare(ctx, 5, hostileSpec(), source)
		prepared <- err
	}()
	registered := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.execs[5] != nil
	}
	for start := time.Now(); !registered(); time.Sleep(10 * time.Millisecond) {
		if time.Since(start) > 5*time.Second {
			t.Fatal("the prepare never started")
		}
	}
	deleted := make(chan struct{})
	go func() {
		c.Delete(ctx, 5) // its removal needs Docker, which api-test lacks; only how long it waits matters here
		close(deleted)
	}()
	select {
	case <-deleted:
	case <-time.After(5 * time.Second):
		t.Fatal("DELETE waited for the upload to finish")
	}
	stall.Close()
	if err := <-prepared; err == nil {
		t.Error("Prepare() succeeded through DELETE")
	}
}

func TestPrepareSourceFailure(t *testing.T) {
	_, c := newRunner(t)
	broken := io.MultiReader(bytes.NewReader(tarball(t, file("r/Dockerfile", "FROM scratch"))[:20]),
		iotest.ErrReader(errors.New("download broke")))
	_, err := c.Prepare(context.Background(), 6, hostileSpec(), broken)
	if se := (*SourceError)(nil); !errors.As(err, &se) || !strings.Contains(err.Error(), "download broke") {
		t.Errorf("Prepare() with a broken source = %#v, want *SourceError", err)
	}
}

func TestClientTransportFailure(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // a runner that went away
	c := &Client{URL: srv.URL, Token: "t", HTTPClient: http.DefaultClient}
	err := c.Delete(context.Background(), 1)
	if err == nil || errors.As(err, new(*Error)) || errors.As(err, new(*Failure)) {
		t.Errorf("Delete() on a closed runner = %#v, want a transport error, not a runner answer", err)
	}
	_, err = c.Prepare(context.Background(), 1, hostileSpec(), bytes.NewReader(tarball(t)))
	if err == nil || errors.As(err, new(*Error)) || errors.As(err, new(*Failure)) {
		t.Errorf("Prepare() on a closed runner = %#v, want a transport error", err)
	}
}
