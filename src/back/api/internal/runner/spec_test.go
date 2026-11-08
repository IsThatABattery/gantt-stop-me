package runner

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	portOf := func(p int) *int { return &p }
	cases := []struct {
		name   string
		change func(*Spec)
		want   string // in the problem
	}{
		{"no project", func(s *Spec) { s.Project = 0 }, "project"},
		{"workflow with capitals", func(s *Spec) { s.Workflow = "Web" }, "workflow"},
		{"workflow of one letter", func(s *Spec) { s.Workflow = "w" }, "workflow"},
		{"workflow with a slash", func(s *Spec) { s.Workflow = "web/../x" }, "workflow"},
		{"workflow ending in -", func(s *Spec) { s.Workflow = "web-" }, "workflow"}, // gantt-p3-web- is no image name
		{"workflow of 33 characters", func(s *Spec) { s.Workflow = strings.Repeat("w", 33) }, "workflow"},
		{"context outside", func(s *Spec) { s.Context = "../other" }, "context"},
		{"absolute context", func(s *Spec) { s.Context = "/etc" }, "context"},
		{"empty context", func(s *Spec) { s.Context = "" }, "context"},
		{"dockerfile outside", func(s *Spec) { s.Dockerfile = "../../etc/passwd" }, "dockerfile"},
		{"absolute dockerfile", func(s *Spec) { s.Dockerfile = "/proc/self/environ" }, "dockerfile"},
		{"relative cache", func(s *Spec) { s.Cache = []string{"cache"} }, "cache"},
		{"root cache", func(s *Spec) { s.Cache = []string{"/"} }, "cache"},
		{"unclean cache", func(s *Spec) { s.Cache = []string{"/a/../var/run"} }, "cache"},
		{"cache adding mount fields", func(s *Spec) { s.Cache = []string{"/a,type=bind,src=/var/run/docker.sock"} }, "cache"},
		{"cache with a colon", func(s *Spec) { s.Cache = []string{"/a:/b"} }, "cache"},
		{"port 0", func(s *Spec) { s.Port = portOf(0) }, "port"},
		{"port too high", func(s *Spec) { s.Port = portOf(70000) }, "port"},
		{"service named app", func(s *Spec) { s.Services[0].Name = "app" }, "reserved"},
		{"service named browser", func(s *Spec) { s.Services[0].Name = "browser" }, "reserved"},
		{"service with a bad name", func(s *Spec) { s.Services[0].Name = "db_1" }, "name"},
		{"service ending in -", func(s *Spec) { s.Services[1].Name = "api-" }, "name"},
		{"two services of one name", func(s *Spec) { s.Services[1].Name = "db" }, "twice"},
		{"image that is a flag", func(s *Spec) { s.Services[0].Image = "--privileged" }, "image"},
		{"image with a flag after it", func(s *Spec) { s.Services[0].Image = "postgres:18 --privileged" }, "image"},
		{"service context outside", func(s *Spec) { s.Services[1].Context = "../.." }, "context"},
		{"service dockerfile outside", func(s *Spec) { s.Services[1].Dockerfile = "../Dockerfile" }, "dockerfile"},
		{"service port", func(s *Spec) { s.Services[1].Port = portOf(-1) }, "port"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := hostileSpec()
			c.change(&s)
			if got := s.validate(); !strings.Contains(got, c.want) {
				t.Errorf("validate() = %q, want a problem with %q", got, c.want)
			}
		})
	}
	if got := hostileSpec().validate(); got != "" {
		t.Errorf("validate() of a good spec = %q", got)
	}
}

func TestValidPrefix(t *testing.T) {
	for p, want := range map[string]bool{"gantt": true, "gantt-dev": true, "g2": true, "": false, "Gantt": false, "gantt-": false, "gantt_x": false} {
		if got := ValidPrefix(p); got != want {
			t.Errorf("ValidPrefix(%q) = %v, want %v", p, got, want)
		}
	}
}
