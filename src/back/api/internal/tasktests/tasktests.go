// Package tasktests holds a task's tests and their Tests section in an issue body: parsing it, rendering it from the
// tests, putting it in a body, and checking a list of tests.
package tasktests

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Test is one of a task's tests.
type Test struct {
	Name     string  `json:"name"`     // its item's line after the bullet, checkbox and pin; a code span script's command
	Kind     string  `json:"kind"`     // "script" or "check"
	Command  string  `json:"command"`  // a script's command, "" for a check
	Details  string  `json:"details"`  // a check's indented block, with its common indent removed; "" when none
	Workflow *string `json:"workflow"` // the workflow its [name] pin names, as written, or nil. It may name no workflow.
}

// NamePattern is the rule for workflow and sidecar names, which a test's [name] pin also follows. A name ends in a
// letter or digit because the runner puts it last in image names and hostnames, where a trailing - is invalid.
const NamePattern = `[a-z][a-z0-9-]{0,30}[a-z0-9]`

var nameRule = regexp.MustCompile(`^` + NamePattern + `$`)

// maxTests is the most tests one task may have.
const maxTests = 64

// maxRendered is the most bytes a task's rendered tests may take, within GitHub's 65,536 characters for a body.
const maxRendered = 64 << 10

// pinRule matches a [name] pin, which must end the line or be followed by whitespace, so a Markdown link such as
// [docs](https://…) stays text.
var pinRule = regexp.MustCompile(`^\[(` + NamePattern + `)\](\s|$)`)

// Parse parses the Tests section of an issue body into its tests, in order: none, and no error, when the body has
// no section or the section has no items, and an error for more than 64.
func Parse(body string) ([]Test, error) {
	type item struct {
		line  string
		block []string
	}
	var items []item
	lines := strings.Split(strings.ReplaceAll(Section(body), "\r\n", "\n"), "\n")
	for _, l := range lines[1:] { // after the heading
		switch {
		case strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* "):
			items = append(items, item{line: l[2:]})
		case len(items) > 0 && (strings.TrimSpace(l) == "" || l[0] == ' ' || l[0] == '\t'):
			items[len(items)-1].block = append(items[len(items)-1].block, l)
		}
	}
	if len(items) > maxTests {
		return nil, fmt.Errorf("the Tests section has %d tests, more than %d", len(items), maxTests)
	}
	tests := make([]Test, len(items)) // [], not null, for none
	for i, it := range items {
		tests[i] = newTest(it.line, dedent(it.block))
	}
	return tests, nil
}

// Section returns the Tests section of body as it stands there: from its "## Tests" heading's line through its last
// line that is not blank, with that line's end, or "" when body has none. The section ends at the next level-1 or
// level-2 heading. A line inside a code fence that opens at column 0 is code, so it neither starts nor ends it.
func Section(body string) string {
	start, end := bounds(body)
	return body[start:end]
}

// bounds returns where Section's text starts and ends in body, by byte offset, or 0, 0 when body has none.
func bounds(body string) (start, end int) {
	section := false
	fenced := "" // the mark of the column-0 code fence the line is in, whose lines are code, not headings
	at := 0
	for raw := range strings.SplitAfterSeq(body, "\n") {
		l := strings.TrimSuffix(raw, "\n")
		if len(l) < len(raw) {
			l = strings.TrimSuffix(l, "\r")
		}
		code := fenced != ""
		if !code {
			fenced = fenceMark(l)
		} else if closes(l, fenced) {
			fenced = ""
		}
		switch {
		case !section:
			if !code && strings.EqualFold(strings.TrimRight(l, " "), "## Tests") {
				section, start, end = true, at, at+len(raw)
			}
		case !code && (l == "#" || l == "##" || strings.HasPrefix(l, "# ") || strings.HasPrefix(l, "## ")):
			return start, end // a level-1 or level-2 heading ends the section
		case strings.TrimSpace(l) != "":
			end = at + len(raw)
		}
		at += len(raw)
	}
	return start, end
}

// Render returns the Tests section of tests, "" for none. Each test is an unticked checklist item after its pin: a
// script whose name is its command as one inline code span, any other script as its name over an sh fence, and a
// check as its name over its indented details.
func Render(tests []Test) string {
	if len(tests) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Tests\n")
	for _, t := range tests {
		b.WriteString("- [ ] ")
		if t.Workflow != nil {
			b.WriteString("[" + *t.Workflow + "] ")
		}
		line, block := t.Name, t.Details
		switch {
		case t.Kind == "script" && t.Name == t.Command:
			line, block = codeSpanOf(t.Command), ""
		case t.Kind == "script":
			mark := "```"
			for strings.Contains(t.Command, mark) { // so no line of the command closes the fence
				mark += "`"
			}
			block = mark + "sh\n" + t.Command + "\n" + mark
		}
		b.WriteString(line + "\n")
		if block == "" {
			continue
		}
		for l := range strings.SplitSeq(block, "\n") {
			if strings.TrimSpace(l) != "" {
				b.WriteString("  " + l)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Replace returns body with its Tests section replaced by Render(tests), appended at the end when it has none, and
// removed when tests is empty. Text outside the section stays as it is, byte for byte.
// shortcut: a section appended to a body that ends inside an open code fence is code, which Parse does not read;
// close the fence first if bodies like that turn up.
func Replace(body string, tests []Test) string {
	start, end := bounds(body)
	if start < end {
		return body[:start] + Render(tests) + body[end:]
	}
	if len(tests) == 0 {
		return body
	}
	if body != "" && !strings.HasSuffix(body, "\n\n") {
		body = strings.TrimSuffix(body, "\n") + "\n\n"
	}
	return body + Render(tests)
}

// Validate checks tests can be a task's: at most 64; each named on one line; of kind script, with a command that is
// not blank, or check; pinned, if at all, to a name that follows the workflow name rule; with no NUL, which the
// database cannot store; and at most 64 KiB rendered. Its error is a sentence naming the first problem.
func Validate(tests []Test) error {
	if len(tests) > maxTests {
		return fmt.Errorf("A task can have at most %d tests.", maxTests)
	}
	for i, t := range tests {
		pin := ""
		if t.Workflow != nil {
			pin = *t.Workflow
		}
		switch n := i + 1; {
		case strings.TrimSpace(t.Name) == "":
			return fmt.Errorf("Test %d needs a name.", n)
		case strings.ContainsAny(t.Name, "\r\n"):
			return fmt.Errorf("Test %d's name must be one line.", n)
		case t.Kind != "script" && t.Kind != "check":
			return fmt.Errorf(`Test %d's kind must be "script" or "check".`, n)
		case t.Kind == "script" && strings.TrimSpace(t.Command) == "":
			return fmt.Errorf("Test %d is a script, so it needs a command.", n)
		case t.Workflow != nil && !nameRule.MatchString(pin):
			return fmt.Errorf("Test %d's workflow must be 2 to 32 lowercase letters, digits or -, starting with a letter and ending in a letter or digit.", n)
		case strings.ContainsRune(t.Name+t.Command+t.Details+pin, 0):
			return fmt.Errorf("Test %d holds a NUL character.", n)
		}
	}
	if len(Render(tests)) > maxRendered {
		return errors.New("The tests take more than 64 KiB.")
	}
	return nil
}

// newTest makes a test from its item's line after the bullet and its dedented block.
func newTest(line, block string) Test {
	t := Test{Kind: "check", Details: block}
	line = strings.TrimLeft(line, " \t")
	for _, box := range []string{"[ ]", "[x]", "[X]"} {
		if rest, ok := strings.CutPrefix(line, box); ok {
			line = strings.TrimLeft(rest, " \t")
			break
		}
	}
	if m := pinRule.FindStringSubmatch(line); m != nil {
		t.Workflow, line = &m[1], line[len(m[0]):]
	}
	t.Name = strings.TrimSpace(line)
	// A blank command would pass without testing anything, so it is left to the LLM.
	if cmd, ok := fence(block); ok && strings.TrimSpace(cmd) != "" {
		t.Kind, t.Command, t.Details = "script", cmd, ""
	} else if cmd, ok := codeSpan(t.Name); ok && block == "" && strings.TrimSpace(cmd) != "" {
		t.Kind, t.Command, t.Name = "script", cmd, cmd
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
	mark := fenceMark(lines[0])
	if len(lines) < 2 || mark == "" {
		return "", false
	}
	if info := strings.Fields(lines[0][len(mark):]); len(info) > 0 && !slices.Contains([]string{"sh", "bash", "shell"}, info[0]) {
		return "", false
	}
	for i, l := range lines[1:] {
		if closes(l, mark) {
			return strings.Join(lines[1:i+1], "\n"), i+2 == len(lines)
		}
	}
	return "", false // never closed
}

// fenceMark returns the run of backticks or tildes that opens a code fence on line l, or "" if l opens none.
func fenceMark(l string) string {
	if !strings.HasPrefix(l, "```") && !strings.HasPrefix(l, "~~~") {
		return ""
	}
	return l[:len(l)-len(strings.TrimLeft(l, l[:1]))]
}

// closes reports whether line l closes the fence opened by mark: a run of mark's character, at least as long, then
// only spaces.
func closes(l, mark string) bool {
	l = strings.TrimRight(l, " \t")
	return len(l) >= len(mark) && strings.Trim(l, mark[:1]) == ""
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

// codeSpanOf returns s as one inline code span that codeSpan reads back: between runs of backticks longer than any
// in s, with a space inside each end when s starts or ends with a backtick.
func codeSpanOf(s string) string {
	mark := "`"
	for strings.Contains(s, mark) {
		mark += "`"
	}
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		s = " " + s + " "
	}
	return mark + s + mark
}
