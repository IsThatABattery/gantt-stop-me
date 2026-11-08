package runner

import (
	"context"
	"errors"
	"testing"
)

func TestBrowserWantsAPort(t *testing.T) {
	s := &Server{}
	c := fakeExecution(t, s, nil)
	s.execs[1].spec.Port = nil
	err := c.Browser(context.Background(), 1)
	if e := (*Error)(nil); !errors.As(err, &e) || e.Status != 409 || e.Code != "no_port" {
		t.Errorf("Browser() without a port = %v, want 409 no_port", err)
	}
}
