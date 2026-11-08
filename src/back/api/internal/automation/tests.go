package automation

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// ErrNoTests is ParseTests' error for an issue body with no Tests section, or a section with no items.
var ErrNoTests = errors.New("the issue has no Tests section")

// maxTests is the most tests one issue may have.
const maxTests = 64

// Test is one item of the Tests section of a task's issue.
type Test struct {
	Position int    // 1-based order in the section, as the decision model numbers tests
	Text     string // the item's line after its bullet, checkbox and pin, trimmed
	Block    string // the indented and blank lines under the item, with their common indent removed
	Pin      string // the workflow named by a [name] pin, as written, or "". It may name no workflow.
	Kind     string // "script" or "llm"
	Command  string // a script's command, or "" for an LLM check
}

// pinRule matches a [name] pin, which must end the line or be followed by whitespace, so a Markdown link such as
// [docs](https://…) stays text.
var pinRule = regexp.MustCompile(`^\[(` + namePattern + `)\](\s|$)`)

// ParseTests parses the Tests section of an issue body into its tests, in order. It returns ErrNoTests when the
// body has no section or the section has no items, and an error for more than maxTests items.
func ParseTests(body string) ([]Test, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	start := slices.IndexFunc(lines, func(l string) bool { return strings.EqualFold(strings.TrimRight(l, " "), "## Tests") })
	if start < 0 {
		return nil, ErrNoTests
	}
	type item struct {
		line  string
		block []string
	}
	var items []item
	for _, l := range lines[start+1:] {
		if l == "#" || l == "##" || strings.HasPrefix(l, "# ") || strings.HasPrefix(l, "## ") {
			break // a level-1 or level-2 heading ends the section
		}
		if strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ") {
			items = append(items, item{line: l[2:]})
		} else if len(items) > 0 && (strings.TrimSpace(l) == "" || l[0] == ' ' || l[0] == '\t') {
			items[len(items)-1].block = append(items[len(items)-1].block, l)
		}
	}
	if len(items) == 0 {
		return nil, ErrNoTests
	}
	if len(items) > maxTests {
		return nil, fmt.Errorf("the Tests section has %d tests, more than %d", len(items), maxTests)
	}
	tests := make([]Test, len(items))
	for i, it := range items {
		tests[i] = newTest(i+1, it.line, dedent(it.block))
	}
	return tests, nil
}

// newTest makes the test at position pos from its line after the bullet and its dedented block.
func newTest(pos int, line, block string) Test {
	t := Test{Position: pos, Block: block, Kind: "llm"}
	line = strings.TrimLeft(line, " \t")
	for _, box := range []string{"[ ]", "[x]", "[X]"} {
		if rest, ok := strings.CutPrefix(line, box); ok {
			line = strings.TrimLeft(rest, " \t")
			break
		}
	}
	if m := pinRule.FindStringSubmatch(line); m != nil {
		t.Pin, line = m[1], line[len(m[0]):]
	}
	t.Text = strings.TrimSpace(line)
	cmd, ok := fence(block)
	if !ok && block == "" {
		cmd, ok = codeSpan(t.Text)
	}
	// A blank command would pass without testing anything, so it is left to the LLM.
	if ok && strings.TrimSpace(cmd) != "" {
		t.Kind, t.Command = "script", cmd
	}
	return t
}

// dedent joins lines with their common indent removed, blank lines emptied, and blank lines at either end dropped.
func dedent(lines []string) string {
	indent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			if n := len(l) - len(strings.TrimLeft(l, " \t")); indent < 0 || n < indent {
				indent = n
			}
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			out[i] = l[indent:]
		}
	}
	return strings.Trim(strings.Join(out, "\n"), "\n")
}

// fence returns the contents of block when block is exactly one fenced code block tagged sh, bash, shell or
// untagged.
func fence(block string) (string, bool) {
	lines := strings.Split(block, "\n")
	open := lines[0]
	if len(lines) < 2 || !strings.HasPrefix(open, "```") && !strings.HasPrefix(open, "~~~") {
		return "", false
	}
	mark := open[:len(open)-len(strings.TrimLeft(open, open[:1]))]
	if info := strings.Fields(open[len(mark):]); len(info) > 0 && !slices.Contains([]string{"sh", "bash", "shell"}, info[0]) {
		return "", false
	}
	for i, l := range lines[1:] {
		// A closing fence is a run of the opening's character, at least as long, then only spaces.
		if l = strings.TrimRight(l, " \t"); len(l) >= len(mark) && strings.Trim(l, mark[:1]) == "" {
			return strings.Join(lines[1:i+1], "\n"), i+2 == len(lines)
		}
	}
	return "", false // never closed
}

// codeSpan returns the contents of text when text is exactly one inline code span, trimmed.
func codeSpan(text string) (string, bool) {
	n := len(text) - len(strings.TrimLeft(text, "`"))
	if n == 0 || len(text) <= 2*n || !strings.HasSuffix(text, strings.Repeat("`", n)) || text[len(text)-n-1] == '`' {
		return "", false
	}
	inner := text[n : len(text)-n]
	// A run of exactly n backticks inside would close the span early, making two spans.
	for run := range strings.FieldsFuncSeq(inner, func(r rune) bool { return r != '`' }) {
		if len(run) == n {
			return "", false
		}
	}
	return strings.TrimSpace(inner), true
}
