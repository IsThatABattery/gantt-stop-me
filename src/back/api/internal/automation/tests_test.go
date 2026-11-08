package automation_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/automation"
)

// exampleTests is the example Tests section from AUTOMATION.md, verbatim.
var exampleTests = strings.Join([]string{
	"## Tests",
	"- [ ] `go test ./internal/board -run Move`",
	"- [ ] [backend] Store and board tests pass together",
	"  ```sh",
	"  go vet ./...",
	"  go test ./internal/store ./internal/board",
	"  ```",
	"- [ ] Signing up with an empty email answers 400 with code invalid_email",
	"- [ ] Dragging a card to Closed closes the issue on GitHub",
	"  1. Sign up and add a project.",
	"  2. Drag any card to Closed.",
	"  3. The card stays in Closed after a reload.",
	"- [ ] The empty board looks right on a phone-sized screen",
}, "\n")

func TestParseTestsExample(t *testing.T) {
	got, err := automation.ParseTests("Sign-up must reject empty emails.\n\n" + exampleTests + "\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []automation.Test{
		{Position: 1, Text: "`go test ./internal/board -run Move`", Kind: "script", Command: "go test ./internal/board -run Move"},
		{Position: 2, Text: "Store and board tests pass together", Pin: "backend", Kind: "script",
			Block:   "```sh\ngo vet ./...\ngo test ./internal/store ./internal/board\n```",
			Command: "go vet ./...\ngo test ./internal/store ./internal/board"},
		{Position: 3, Text: "Signing up with an empty email answers 400 with code invalid_email", Kind: "llm"},
		{Position: 4, Text: "Dragging a card to Closed closes the issue on GitHub", Kind: "llm",
			Block: "1. Sign up and add a project.\n2. Drag any card to Closed.\n3. The card stays in Closed after a reload."},
		{Position: 5, Text: "The empty board looks right on a phone-sized screen", Kind: "llm"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ParseTests =\n%+v\nwant\n%+v", got, want)
	}
}

// section returns a body whose Tests section holds lines.
func section(lines ...string) string {
	return "## Tests\n" + strings.Join(lines, "\n")
}

// llm is an LLM check with text and block.
func llm(pos int, text, block string) automation.Test {
	return automation.Test{Position: pos, Text: text, Block: block, Kind: "llm"}
}

// script is a script with text, block and command.
func script(text, block, command string) automation.Test {
	return automation.Test{Position: 1, Text: text, Block: block, Kind: "script", Command: command}
}

// pinned is t pinned to pin.
func pinned(t automation.Test, pin string) automation.Test {
	t.Pin = pin
	return t
}

func TestParseTests(t *testing.T) {
	sh := "```sh\necho hi\n```"
	for _, tc := range []struct {
		name string
		body string
		want []automation.Test
	}{
		// Section
		{"heading in any case", "## TESTS\n- a", []automation.Test{llm(1, "a", "")}},
		{"heading with trailing spaces", "## Tests   \n- a", []automation.Test{llm(1, "a", "")}},
		{"text before the heading", "Intro\n- not a test\n\n## Tests\n- a", []automation.Test{llm(1, "a", "")}},
		{"ends at a level-2 heading", section("- a", "## Notes", "- b"), []automation.Test{llm(1, "a", "")}},
		{"ends at a level-1 heading", section("- a", "# Notes", "- b"), []automation.Test{llm(1, "a", "")}},
		{"ends at a bare #", section("- a", "#", "- b"), []automation.Test{llm(1, "a", "")}},
		{"ends at a bare ##", section("- a", "##", "- b"), []automation.Test{llm(1, "a", "")}},
		{"first section only", section("- a", "## Tests", "- b"), []automation.Test{llm(1, "a", "")}},
		{"level-3 heading inside", section("- a", "### More", "- b"), []automation.Test{llm(1, "a", ""), llm(2, "b", "")}},
		{"hashtag inside", section("- a", "#tag", "- b"), []automation.Test{llm(1, "a", ""), llm(2, "b", "")}},
		{"indented heading inside", section("- a", " ## Notes"), []automation.Test{llm(1, "a", "## Notes")}},
		{"CRLF line endings", "## Tests\r\n- a\r\n  one\r\n  two\r\n", []automation.Test{llm(1, "a", "one\ntwo")}},

		// Items and blocks
		{"star bullet", section("* a"), []automation.Test{llm(1, "a", "")}},
		{"other lines ignored", section("intro", "  indented intro", "- a", "+ b", "-c", "1. d"), []automation.Test{llm(1, "a", "")}},
		{"indented bullet in the block", section("- a", "  - b"), []automation.Test{llm(1, "a", "- b")}},
		{"block after an ignored line", section("- a", "lazy", "  more"), []automation.Test{llm(1, "a", "more")}},
		{"common indent removed", section("- a", "", "    one", "      two", "", "    three", "", ""),
			[]automation.Test{llm(1, "a", "one\n  two\n\nthree")}},
		{"tab indent", section("- a", "\tone"), []automation.Test{llm(1, "a", "one")}},
		{"blocks split by items", section("- a", "  one", "- b", "  two"), []automation.Test{llm(1, "a", "one"), llm(2, "b", "two")}},

		// Checkbox, pin and text
		{"no checkbox", section("- a"), []automation.Test{llm(1, "a", "")}},
		{"ticked checkbox", section("- [x] a"), []automation.Test{llm(1, "a", "")}},
		{"capital ticked checkbox", section("- [X] a"), []automation.Test{llm(1, "a", "")}},
		{"other box kept", section("- [y] a"), []automation.Test{llm(1, "[y] a", "")}},
		{"pin", section("- [ ] [backend] a"), []automation.Test{pinned(llm(1, "a", ""), "backend")}},
		{"pin without checkbox", section("- [frontend] a"), []automation.Test{pinned(llm(1, "a", ""), "frontend")}},
		{"pin ending the line", section("- [ ] [backend]"), []automation.Test{pinned(llm(1, "", ""), "backend")}},
		{"pin kept as written", section("- [no-such-workflow] a"), []automation.Test{pinned(llm(1, "a", ""), "no-such-workflow")}},
		{"only one pin", section("- [backend] [frontend] a"), []automation.Test{pinned(llm(1, "[frontend] a", ""), "backend")}},
		{"link is text", section("- [ ] [docs](https://example.com) say so"), []automation.Test{llm(1, "[docs](https://example.com) say so", "")}},
		{"capital name is text", section("- [Backend] a"), []automation.Test{llm(1, "[Backend] a", "")}},
		{"one letter is text", section("- [ ] [b] a"), []automation.Test{llm(1, "[b] a", "")}},
		{"pin glued to text is text", section("- [backend]a"), []automation.Test{llm(1, "[backend]a", "")}},
		{"text trimmed", section("- [ ]   a  b  "), []automation.Test{llm(1, "a  b", "")}},

		// Kinds
		{"code span", section("- [ ] `go test ./...`"), []automation.Test{script("`go test ./...`", "", "go test ./...")}},
		{"pinned code span", section("- [backend] `make`"), []automation.Test{pinned(script("`make`", "", "make"), "backend")}},
		{"double-backtick span", section("- `` echo `date` ``"), []automation.Test{script("`` echo `date` ``", "", "echo `date`")}},
		{"two code spans", section("- `a` `b`"), []automation.Test{llm(1, "`a` `b`", "")}},
		{"code span in text", section("- run `make`"), []automation.Test{llm(1, "run `make`", "")}},
		{"code span with block", section("- `make`", "  then look"), []automation.Test{llm(1, "`make`", "then look")}},
		{"blank code span", section("- ` `"), []automation.Test{llm(1, "` `", "")}},
		{"unbalanced code span", section("- `a``"), []automation.Test{llm(1, "`a``", "")}},
		{"sh fence", section("- a", "  ```sh", "  echo hi", "  ```"), []automation.Test{script("a", sh, "echo hi")}},
		{"bash fence", section("- a", "  ```bash", "  echo hi", "  ```"), []automation.Test{script("a", "```bash\necho hi\n```", "echo hi")}},
		{"shell fence", section("- a", "  ```shell", "  echo hi", "  ```"), []automation.Test{script("a", "```shell\necho hi\n```", "echo hi")}},
		{"untagged fence", section("- a", "  ```", "  echo hi", "  ```"), []automation.Test{script("a", "```\necho hi\n```", "echo hi")}},
		{"tilde fence", section("- a", "  ~~~sh", "  echo hi", "  ~~~"), []automation.Test{script("a", "~~~sh\necho hi\n~~~", "echo hi")}},
		{"fence with more info", section("- a", "  ```sh title", "  echo hi", "  ```"), []automation.Test{script("a", "```sh title\necho hi\n```", "echo hi")}},
		{"longer closing fence", section("- a", "  ```", "  echo hi", "  `````  "), []automation.Test{script("a", "```\necho hi\n`````  ", "echo hi")}},
		{"fence keeps inner indent", section("- a", "  ```sh", "  if true; then", "    echo hi", "  fi", "", "  ```"),
			[]automation.Test{script("a", "```sh\nif true; then\n  echo hi\nfi\n\n```", "if true; then\n  echo hi\nfi\n")}},
		{"fence with a code span text", section("- `x`", "  "+strings.ReplaceAll(sh, "\n", "\n  ")), []automation.Test{script("`x`", sh, "echo hi")}},
		{"python fence", section("- a", "  ```python", "  print(1)", "  ```"), []automation.Test{llm(1, "a", "```python\nprint(1)\n```")}},
		{"fence then text", section("- a", "  ```sh", "  echo hi", "  ```", "  then look"), []automation.Test{llm(1, "a", sh+"\nthen look")}},
		{"text then fence", section("- a", "  first", "  ```sh", "  echo hi", "  ```"), []automation.Test{llm(1, "a", "first\n"+sh)}},
		{"two fences", section("- a", "  ```sh", "  echo hi", "  ```", "  ```sh", "  echo hi", "  ```"),
			[]automation.Test{llm(1, "a", sh+"\n"+sh)}},
		{"unclosed fence", section("- a", "  ```sh", "  echo hi"), []automation.Test{llm(1, "a", "```sh\necho hi")}},
		{"tilde does not close backticks", section("- a", "  ```", "  echo hi", "  ~~~"), []automation.Test{llm(1, "a", "```\necho hi\n~~~")}},
		{"short closing fence", section("- a", "  ````", "  echo hi", "  ```"), []automation.Test{llm(1, "a", "````\necho hi\n```")}},
		{"empty fence", section("- a", "  ```sh", "  ```"), []automation.Test{llm(1, "a", "```sh\n```")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := automation.ParseTests(tc.body)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("ParseTests = %+v, %v\nwant %+v", got, err, tc.want)
			}
		})
	}
}

func TestParseTestsNone(t *testing.T) {
	for _, body := range []string{
		"",
		"No tests here.\n- a",
		"### Tests\n- a",
		" ## Tests\n- a",
		"##Tests\n- a",
		"## Tests to run\n- a",
		"## Tests",
		"## Tests\nJust a paragraph.\n  indented\n",
		"## Tests\n## Notes\n- a",
	} {
		if got, err := automation.ParseTests(body); !errors.Is(err, automation.ErrNoTests) {
			t.Errorf("ParseTests(%q) = %+v, %v; want ErrNoTests", body, got, err)
		}
	}
}

func TestParseTestsLimit(t *testing.T) {
	var items []string
	for i := 1; i <= 64; i++ {
		items = append(items, fmt.Sprintf("- [ ] test %d", i))
	}
	got, err := automation.ParseTests(section(items...))
	if err != nil || len(got) != 64 || got[63].Position != 64 || got[63].Text != "test 64" {
		t.Fatalf("64 tests: %d, %v", len(got), err)
	}
	_, err = automation.ParseTests(section(append(items, "- [ ] test 65")...))
	if err == nil || errors.Is(err, automation.ErrNoTests) || err.Error() != "the Tests section has 65 tests, more than 64" {
		t.Fatalf("65 tests: %v", err)
	}
}
