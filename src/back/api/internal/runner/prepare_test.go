package runner

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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
