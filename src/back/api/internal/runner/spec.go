// Package runner builds and runs a workflow execution's containers on the API's request, and holds the API's client for it.
// It is the only part of the app with Docker access, and it shells out to the docker CLI for every Docker operation.
package runner

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// Spec is a workflow after defaults and validation, plus the execution's project. The API sends it as the spec part.
type Spec struct {
	Project    int64             `json:"project"`
	Workflow   string            `json:"workflow"`
	Context    string            `json:"context"`    // relative to the repository root
	Dockerfile string            `json:"dockerfile"` // relative to Context
	Target     string            `json:"target"`     // "" builds the last stage
	Env        map[string]string `json:"env"`
	Cache      []string          `json:"cache"` // absolute paths in test containers, each kept in a volume
	Port       *int              `json:"port"`  // the app's port; nil runs no app
	Services   []Service         `json:"services"`
}

// Service is a sidecar: an image, or a build from Context, Dockerfile and Target.
type Service struct {
	Name       string            `json:"name"`
	Image      string            `json:"image,omitempty"`
	Context    string            `json:"context"`
	Dockerfile string            `json:"dockerfile"`
	Target     string            `json:"target,omitempty"`
	Env        map[string]string `json:"env"`
	Health     string            `json:"health,omitempty"` // run with sh -c until it exits 0
	Port       *int              `json:"port"`             // dialled until it accepts
}

var (
	// name is a workflow or sidecar name, which also becomes part of image, container and volume names.
	// It ends in a letter or digit, as an image name's last part must, the same rule as the API's parser.
	name = regexp.MustCompile(`^[a-z][a-z0-9-]{0,30}[a-z0-9]$`)
	// image is a registry reference. It cannot start with "-", so docker never reads it as a flag.
	image = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,254}$`)
	// cachePath has no commas, "=" or quotes, so it cannot add fields to the --mount value it goes into.
	cachePath = regexp.MustCompile(`^(/[a-zA-Z0-9._@+-]+)+$`)
)

// nameRule says what name accepts.
const nameRule = "2 to 32 lowercase letters, digits or -, starting with a letter and ending in a letter or digit"

// ValidPrefix reports whether p can be RUN_PREFIX. It starts every name the runner makes, so it follows the name rule.
func ValidPrefix(p string) bool { return name.MatchString(p) }

// validate re-checks what the runner's safety depends on, whoever produced the spec. It returns the problem, or "".
func (s Spec) validate() string {
	if s.Project < 1 {
		return "project must be a positive id"
	}
	if !name.MatchString(s.Workflow) {
		return "workflow must be " + nameRule
	}
	if msg := buildPaths("", s.Context, s.Dockerfile); msg != "" {
		return msg
	}
	for _, p := range s.Cache {
		if !cachePath.MatchString(p) || filepath.Clean(p) != p {
			return fmt.Sprintf("cache path %q must be absolute and clean, of letters, digits and ._@+-", p)
		}
	}
	if msg := port("", s.Port); msg != "" {
		return msg
	}
	seen := map[string]bool{}
	for _, svc := range s.Services {
		switch {
		case !name.MatchString(svc.Name):
			return fmt.Sprintf("service %q: name must be %s", svc.Name, nameRule)
		case svc.Name == "app" || svc.Name == "browser":
			return fmt.Sprintf("service %q: the name is reserved", svc.Name)
		case seen[svc.Name]:
			return fmt.Sprintf("service %q: the name is used twice", svc.Name)
		case svc.Image != "" && !image.MatchString(svc.Image):
			return fmt.Sprintf("service %q: image is not a registry reference", svc.Name)
		}
		seen[svc.Name] = true
		if svc.Image == "" {
			if msg := buildPaths("service "+svc.Name+": ", svc.Context, svc.Dockerfile); msg != "" {
				return msg
			}
		}
		if msg := port("service "+svc.Name+": ", svc.Port); msg != "" {
			return msg
		}
	}
	return ""
}

// buildPaths checks that a build's context and Dockerfile stay inside the source directory. The Dockerfile is
// relative to the context and may climb out of it, as the API's parser allows, but not out of the repository.
func buildPaths(field, context, dockerfile string) string {
	if !filepath.IsLocal(context) {
		return field + "context must be a relative path inside the repository"
	}
	if filepath.IsAbs(dockerfile) || !filepath.IsLocal(filepath.Join(context, dockerfile)) {
		return field + "dockerfile must be a relative path inside the repository"
	}
	return ""
}

// port checks an optional TCP port.
func port(field string, p *int) string {
	if p != nil && (*p < 1 || *p > 65535) {
		return field + "port must be 1 to 65535"
	}
	return ""
}

// testImage is the tag of the workflow's test image.
func (s *Server) testImage(spec Spec) string {
	return fmt.Sprintf("%s-p%d-%s", s.Prefix, spec.Project, spec.Workflow)
}

// serviceImage is the image a sidecar runs: its own image, or the tag of its build.
// The build tag puts "_" before the service, which names cannot hold, so workflow "web" with sidecar "api"
// never shares a tag with workflow "web-api".
func (s *Server) serviceImage(spec Spec, svc Service) string {
	if svc.Image != "" {
		return svc.Image
	}
	return s.testImage(spec) + "_" + svc.Name
}

// cacheVolume is the named volume that keeps the workflow's nth cache path between runs in this project.
func (s *Server) cacheVolume(spec Spec, n int) string {
	return fmt.Sprintf("%s-p%d-%s-cache%d", s.Prefix, spec.Project, spec.Workflow, n)
}

// network is the execution's internal network.
func (s *Server) network(id int64) string {
	return fmt.Sprintf("%s-x%d", s.Prefix, id)
}

// container is the name of the execution's container with hostname host: app, a sidecar or browser.
func (s *Server) container(id int64, host string) string {
	return s.network(id) + "-" + host
}
