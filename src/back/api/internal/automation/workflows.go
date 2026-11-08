package automation

import (
	"cmp"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"gantt-stop-me/api/internal/tasktests"
)

// Workflow is one workflow of a repository's .gantt/workflows.yml, with its defaults applied. Its JSON is the
// runner's spec without "project".
type Workflow struct {
	Name        string            `json:"workflow"`
	Description string            `json:"-"` // for the decision model; "" only in DefaultWorkflows
	Context     string            `json:"context"`
	Dockerfile  string            `json:"dockerfile"` // relative to Context
	Target      string            `json:"target"`     // "" builds the last stage
	Env         map[string]string `json:"env"`
	Cache       []string          `json:"cache"`
	Port        *int              `json:"port"` // the app's port; nil runs no app
	Services    []Service         `json:"services"`
	Timeout     time.Duration     `json:"-"`
}

// Service is a workflow's sidecar: a pulled Image, or, when Image is "", a build from Context, Dockerfile and Target.
type Service struct {
	Name       string            `json:"name"`
	Image      string            `json:"image,omitempty"`
	Context    string            `json:"context,omitempty"`
	Dockerfile string            `json:"dockerfile,omitempty"`
	Target     string            `json:"target,omitempty"`
	Env        map[string]string `json:"env"`
	Health     string            `json:"health,omitempty"` // run with sh -c until it exits 0
	Port       *int              `json:"port"`             // dialled until it accepts
}

// nameRule is tasktests.NamePattern, the rule for workflow and sidecar names.
var nameRule = regexp.MustCompile(`^` + tasktests.NamePattern + `$`)

// cacheRule is the characters the runner allows in a cache path. The path goes into docker's --mount flag, where
// a comma or = would add mount options, such as a host bind mount.
var cacheRule = regexp.MustCompile(`^(/[A-Za-z0-9._@+-]+)+$`)

// plainKey is a key that a field path shows as it is. Others are quoted, so a key cannot carry a newline or
// Markdown into a message that is posted to GitHub.
var plainKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Limits from the workflow file's rules.
const (
	maxFile        = 64 << 10 // bytes; the whole file becomes nodes before any other check
	maxCache       = 4
	maxServices    = 8
	maxTimeout     = 2 * time.Hour
	defaultTimeout = 30 * time.Minute
)

// DefaultWorkflows returns the workflows of a repository with no workflows file: one, "default", with every field
// at its default.
func DefaultWorkflows() []Workflow {
	return []Workflow{newWorkflow("default")}
}

// newWorkflow returns the workflow name with every field at its default.
func newWorkflow(name string) Workflow {
	return Workflow{Name: name, Context: ".", Dockerfile: "Dockerfile", Env: map[string]string{}, Cache: []string{},
		Services: []Service{}, Timeout: defaultTimeout}
}

// ParseWorkflows parses a workflows file into its workflows, in file order, with defaults applied. An error names
// the field that breaks a rule as a path, such as workflows.frontend.services.api.port, with its line.
//
// It walks yaml.Node rather than loading structs with yaml.WithKnownFields: only nodes keep the file's order, and
// v4 checks unknown and repeated keys only when loading structs and maps, naming Go types instead of field paths.
// So each rejects both itself.
func ParseWorkflows(data []byte) ([]Workflow, error) {
	if len(data) > maxFile {
		return nil, fmt.Errorf("the file: has %d bytes, more than %d", len(data), maxFile)
	}
	var doc yaml.Node
	if err := yaml.Load(data, &doc); err != nil {
		return nil, err
	}
	var ws []Workflow
	images := map[string]string{} // image name → the path of the workflow or sidecar the runner builds it for
	err := each(doc.Content[0], "", func(k, v *yaml.Node, at string) error {
		if k.Value != "workflows" {
			return fail(k, at, "is not a known field")
		}
		return each(v, at, func(k, v *yaml.Node, at string) error {
			w, err := parseWorkflow(k, v, at)
			ws = append(ws, w)
			if err == nil {
				err = claimImages(images, k, w, at)
			}
			return err
		})
	})
	if err == nil && len(ws) == 0 {
		err = errors.New("workflows: needs at least one workflow")
	}
	if err != nil {
		return nil, err
	}
	return ws, nil
}

// claimImages records in images the names the runner builds w's images under: {workflow} for its tests and
// {workflow}-{sidecar} for each built sidecar. Names may contain -, so workflow back's sidecar end and workflow
// back-end would share one, and when both run at once, one would start the other's image.
func claimImages(images map[string]string, k *yaml.Node, w Workflow, at string) error {
	own := [][2]string{{w.Name, at}}
	for _, s := range w.Services {
		if s.Image == "" {
			own = append(own, [2]string{w.Name + "-" + s.Name, join(join(at, "services"), s.Name)})
		}
	}
	for _, im := range own {
		if other, ok := images[im[0]]; ok {
			return fail(k, im[1], "builds an image named %s, as %s does", im[0], other)
		}
		images[im[0]] = im[1]
	}
	return nil
}

// parseWorkflow parses the workflow named by the key k, whose fields v are at path at.
func parseWorkflow(k, v *yaml.Node, at string) (Workflow, error) {
	w := newWorkflow(k.Value)
	if err := checkName(k, at); err != nil {
		return w, err
	}
	var b build
	err := each(v, at, func(k, v *yaml.Node, at string) (err error) {
		switch k.Value {
		case "description":
			w.Description, err = text(v, at)
		case "context", "dockerfile", "target":
			err = b.set(k.Value, v, at)
		case "env":
			w.Env, err = env(v, at)
		case "cache":
			w.Cache, err = cache(v, at)
		case "port":
			w.Port, err = port(v, at)
		case "services":
			w.Services, err = services(v, at)
		case "timeout":
			w.Timeout, err = timeout(v, at)
		default:
			err = fail(k, at, "is not a known field")
		}
		return err
	})
	if err != nil {
		return w, err
	}
	if w.Description = strings.TrimSpace(w.Description); w.Description == "" {
		return w, fail(k, at+".description", "is required")
	}
	err = b.resolve(at)
	w.Context, w.Dockerfile, w.Target = b.context, b.dockerfile, b.target
	return w, err
}

// services parses a workflow's sidecars, in file order, which is the order they start in.
func services(n *yaml.Node, at string) ([]Service, error) {
	ss := []Service{}
	err := each(n, at, func(k, v *yaml.Node, at string) error {
		s, err := service(k, v, at)
		ss = append(ss, s)
		return err
	})
	if err == nil && len(ss) > maxServices {
		err = fail(n, at, "has %d sidecars, more than %d", len(ss), maxServices)
	}
	return ss, err
}

// service parses the sidecar named by the key k, whose fields v are at path at.
func service(k, v *yaml.Node, at string) (Service, error) {
	s := Service{Name: k.Value, Env: map[string]string{}}
	if err := checkName(k, at); err != nil {
		return s, err
	}
	if s.Name == "app" || s.Name == "browser" {
		return s, fail(k, at, "is a reserved name")
	}
	var b build
	var built *yaml.Node // the first build field set, which image rules out
	err := each(v, at, func(k, v *yaml.Node, at string) (err error) {
		switch k.Value {
		case "image":
			// A blank one would build the sidecar instead, and a leading - would read as a flag on the runner's
			// docker command line.
			if s.Image, err = text(v, at); err == nil && (strings.TrimSpace(s.Image) == "" || strings.HasPrefix(s.Image, "-")) {
				err = fail(v, at, "must be an image reference")
			}
		case "context", "dockerfile", "target":
			built = cmp.Or(built, k)
			err = b.set(k.Value, v, at)
		case "env":
			s.Env, err = env(v, at)
		case "health":
			if s.Health, err = text(v, at); err == nil && strings.TrimSpace(s.Health) == "" {
				err = fail(v, at, "must be a command")
			}
		case "port":
			s.Port, err = port(v, at)
		default:
			err = fail(k, at, "is not a known field")
		}
		return err
	})
	switch {
	case err != nil:
	case s.Image != "" && built != nil:
		err = fail(built, join(at, built.Value), "cannot be set with image")
	case s.Image == "":
		err = b.resolve(at)
		s.Context, s.Dockerfile, s.Target = b.context, b.dockerfile, b.target
	}
	return s, err
}

// build is where an image is built from: the context, dockerfile and target fields of a workflow or sidecar.
type build struct {
	context, dockerfile, target string
	dockerfileAt                *yaml.Node // dockerfile's value, for an error's line
}

// set reads the build field key from v, at path at.
func (b *build) set(key string, v *yaml.Node, at string) (err error) {
	switch key {
	case "context":
		if b.context, err = text(v, at); err == nil && !inside(b.context) {
			err = fail(v, at, "must be a relative path inside the repository")
		}
	case "dockerfile":
		b.dockerfileAt = v
		b.dockerfile, err = text(v, at)
	case "target":
		b.target, err = text(v, at)
	}
	return err
}

// resolve applies the defaults, then checks that the Dockerfile, which is relative to the context, stays inside the
// repository. at is the path of the workflow or sidecar.
func (b *build) resolve(at string) error {
	b.context = cmp.Or(b.context, ".")
	b.dockerfile = cmp.Or(b.dockerfile, "Dockerfile")
	if path.IsAbs(b.dockerfile) || !inside(path.Join(b.context, b.dockerfile)) {
		return fail(b.dockerfileAt, at+".dockerfile", "must be a relative path inside the repository")
	}
	return nil
}

// inside reports whether p is a relative path that stays inside the directory it is relative to.
func inside(p string) bool {
	c := path.Clean(p)
	return !path.IsAbs(c) && c != ".." && !strings.HasPrefix(c, "../")
}

// checkName checks the workflow or sidecar name in the key k, at path at.
func checkName(k *yaml.Node, at string) error {
	if !nameRule.MatchString(k.Value) {
		return fail(k, at, "must be 2 to 32 lowercase letters, digits or -, starting with a letter and ending with a letter or digit")
	}
	return nil
}

// env parses a string map. Scalars such as 5432 and true count as their text, and null as "". A key may not be
// empty or hold = or NUL, which docker's -e NAME=value cannot pass.
func env(n *yaml.Node, envAt string) (map[string]string, error) {
	m := map[string]string{}
	err := each(n, envAt, func(k, v *yaml.Node, at string) (err error) {
		if k.Value == "" || strings.ContainsAny(k.Value, "=\x00") {
			return fail(k, envAt, "has a key that is not a variable name")
		}
		m[k.Value], err = text(v, at)
		return err
	})
	return m, err
}

// cache parses a workflow's cache paths: absolute, clean, in cacheRule, not /tmp, which every container already
// mounts as a tmpfs, unique, and at most maxCache of them.
func cache(n *yaml.Node, at string) ([]string, error) {
	cs := []string{}
	if isNull(n) {
		return cs, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, fail(n, at, "must be a list")
	}
	if len(n.Content) > maxCache {
		return nil, fail(n, at, "has %d paths, more than %d", len(n.Content), maxCache)
	}
	for i, item := range n.Content {
		at := fmt.Sprintf("%s[%d]", at, i)
		c, err := text(item, at)
		if err != nil {
			return nil, err
		}
		if !cacheRule.MatchString(c) || path.Clean(c) != c || c == "/tmp" {
			return nil, fail(item, at, "must be a clean absolute path of letters, digits and ._@+-, other than / and /tmp")
		}
		if slices.Contains(cs, c) {
			return nil, fail(item, at, "repeats %s", c)
		}
		cs = append(cs, c)
	}
	return cs, nil
}

// port parses a TCP port, or nil for null.
func port(n *yaml.Node, at string) (*int, error) {
	if isNull(n) {
		return nil, nil
	}
	var p int
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!int" || n.Load(&p) != nil || p < 1 || p > 65535 {
		return nil, fail(n, at, "must be a port from 1 to 65535")
	}
	return &p, nil
}

// timeout parses a Go duration above 0 and at most maxTimeout, or defaultTimeout for null.
func timeout(n *yaml.Node, at string) (time.Duration, error) {
	s, err := text(n, at)
	if err != nil || s == "" {
		return defaultTimeout, err
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 || d > maxTimeout {
		return 0, fail(n, at, "must be a duration above 0 and at most 2h, such as 20m")
	}
	return d, nil
}

// text returns the scalar n as written, or "" for null. Maps and lists are refused, and so is a NUL, such as from
// "\0", which no command line or database column can carry.
func text(n *yaml.Node, at string) (string, error) {
	switch {
	case n.Kind != yaml.ScalarNode:
		return "", fail(n, at, "must be a string")
	case isNull(n):
		return "", nil
	case strings.Contains(n.Value, "\x00"):
		return "", fail(n, at, "holds a NUL character")
	}
	return n.Value, nil
}

// each calls f, in file order, with each key and value of the mapping n at path at, and the value's path. Null
// counts as an empty mapping. Keys must be strings and must not repeat. Values may not be aliases: they would make
// the file hard for the UI to edit, and a small file could expand to a huge one in this walk.
func each(n *yaml.Node, at string, f func(k, v *yaml.Node, at string) error) error {
	if isNull(n) {
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return fail(n, at, "must be a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		kAt := join(at, k.Value)
		switch {
		case k.Kind != yaml.ScalarNode:
			return fail(k, at, "has a key that is not a string")
		case seen[k.Value]:
			return fail(k, kAt, "appears twice")
		case v.Kind == yaml.AliasNode:
			return fail(v, kAt, "is an alias, which this file cannot use")
		}
		seen[k.Value] = true
		if err := f(k, v, kAt); err != nil {
			return err
		}
	}
	return nil
}

// isNull reports whether n is a null scalar, such as an empty value.
func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null"
}

// join returns the path of key under at.
func join(at, key string) string {
	if !plainKey.MatchString(key) {
		key = strconv.Quote(key)
	}
	if at == "" {
		return key
	}
	return at + "." + key
}

// fail reports a rule broken by the field at path at, with the line of its node n.
func fail(n *yaml.Node, at, format string, args ...any) error {
	return fmt.Errorf("%s: %s (line %d)", cmp.Or(at, "the file"), fmt.Sprintf(format, args...), n.Line)
}
