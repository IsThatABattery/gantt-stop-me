package tasktests_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"gantt-stop-me/api/internal/tasktests"
)

// exampleTests is the example under Tests section in docs/api/automation.md, verbatim.
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

// check is a check named name with details.
func check(name, details string) tasktests.Test {
	return tasktests.Test{Name: name, Kind: "check", Details: details}
}

// script is a script named name that runs command.
func script(name, command string) tasktests.Test {
	return tasktests.Test{Name: name, Kind: "script", Command: command}
}

// pinned is t pinned to workflow.
func pinned(t tasktests.Test, workflow string) tasktests.Test {
	t.Workflow = &workflow
	return t
}

func TestParseExample(t *testing.T) {
	got, err := tasktests.Parse("Sign-up must reject empty emails.\n\n" + exampleTests + "\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []tasktests.Test{
		script("go test ./internal/board -run Move", "go test ./internal/board -run Move"),
		pinned(script("Store and board tests pass together", "go vet ./...\ngo test ./internal/store ./internal/board"), "backend"),
		check("Signing up with an empty email answers 400 with code invalid_email", ""),
		check("Dragging a card to Closed closes the issue on GitHub",
			"1. Sign up and add a project.\n2. Drag any card to Closed.\n3. The card stays in Closed after a reload."),
		check("The empty board looks right on a phone-sized screen", ""),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse =\n%+v\nwant\n%+v", got, want)
	}
	if r := tasktests.Render(got); r != exampleTests+"\n" {
		t.Fatalf("Render =\n%s\nwant the example back", r)
	}
}

// section returns a body whose Tests section holds lines.
func section(lines ...string) string {
	return "## Tests\n" + strings.Join(lines, "\n")
}

// Every valid list Parse gives back from a case renders to a section that parses to the same list.
func TestParse(t *testing.T) {
	sh := "```sh\necho hi\n```"
	for _, tc := range []struct {
		name string
		body string
		want []tasktests.Test
	}{
		// Section
		{"heading in any case", "## TESTS\n- a", []tasktests.Test{check("a", "")}},
		{"heading with trailing spaces", "## Tests   \n- a", []tasktests.Test{check("a", "")}},
		{"text before the heading", "Intro\n- not a test\n\n## Tests\n- a", []tasktests.Test{check("a", "")}},
		{"ends at a level-2 heading", section("- a", "## Notes", "- b"), []tasktests.Test{check("a", "")}},
		{"ends at a level-1 heading", section("- a", "# Notes", "- b"), []tasktests.Test{check("a", "")}},
		{"ends at a bare #", section("- a", "#", "- b"), []tasktests.Test{check("a", "")}},
		{"ends at a bare ##", section("- a", "##", "- b"), []tasktests.Test{check("a", "")}},
		{"first section only", section("- a", "## Tests", "- b"), []tasktests.Test{check("a", "")}},
		{"level-3 heading inside", section("- a", "### More", "- b"), []tasktests.Test{check("a", ""), check("b", "")}},
		{"hashtag inside", section("- a", "#tag", "- b"), []tasktests.Test{check("a", ""), check("b", "")}},
		{"indented heading inside", section("- a", " ## Notes"), []tasktests.Test{check("a", "## Notes")}},
		{"CRLF line endings", "## Tests\r\n- a\r\n  one\r\n  two\r\n", []tasktests.Test{check("a", "one\ntwo")}},
		{"heading in a fence does not end it", section("- [ ] builds", "```sh", "# build it", "make", "```", "- [ ] lints"),
			[]tasktests.Test{check("builds", ""), check("lints", "")}},
		{"heading in a tilde fence does not end it", section("- a", "~~~", "## x", "~~~", "- b"),
			[]tasktests.Test{check("a", ""), check("b", "")}},
		{"heading after a fence ends it", section("- a", "````", "```", "# x", "````", "## Notes", "- b"),
			[]tasktests.Test{check("a", "")}},
		{"unclosed fence runs to the end", section("- a", "```", "# x", "- b"), []tasktests.Test{check("a", ""), check("b", "")}},
		{"heading in a fence does not start it", "```md\n## Tests\n- fake\n```\n\n## Tests\n- real", []tasktests.Test{check("real", "")}},

		// Items and blocks
		{"star bullet", section("* a"), []tasktests.Test{check("a", "")}},
		{"other lines ignored", section("intro", "  indented intro", "- a", "+ b", "-c", "1. d"), []tasktests.Test{check("a", "")}},
		{"indented bullet in the block", section("- a", "  - b"), []tasktests.Test{check("a", "- b")}},
		{"block after an ignored line", section("- a", "lazy", "  more"), []tasktests.Test{check("a", "more")}},
		{"common indent removed", section("- a", "", "    one", "      two", "", "    three", "", ""),
			[]tasktests.Test{check("a", "one\n  two\n\nthree")}},
		{"tab indent", section("- a", "\tone"), []tasktests.Test{check("a", "one")}},
		{"blocks split by items", section("- a", "  one", "- b", "  two"), []tasktests.Test{check("a", "one"), check("b", "two")}},

		// Checkbox, pin and text
		{"no checkbox", section("- a"), []tasktests.Test{check("a", "")}},
		{"ticked checkbox", section("- [x] a"), []tasktests.Test{check("a", "")}},
		{"capital ticked checkbox", section("- [X] a"), []tasktests.Test{check("a", "")}},
		{"other box kept", section("- [y] a"), []tasktests.Test{check("[y] a", "")}},
		{"pin", section("- [ ] [backend] a"), []tasktests.Test{pinned(check("a", ""), "backend")}},
		{"pin without checkbox", section("- [frontend] a"), []tasktests.Test{pinned(check("a", ""), "frontend")}},
		{"pin ending the line", section("- [ ] [backend]"), []tasktests.Test{pinned(check("", ""), "backend")}},
		{"pin kept as written", section("- [no-such-workflow] a"), []tasktests.Test{pinned(check("a", ""), "no-such-workflow")}},
		{"only one pin", section("- [backend] [frontend] a"), []tasktests.Test{pinned(check("[frontend] a", ""), "backend")}},
		{"link is text", section("- [ ] [docs](https://example.com) say so"), []tasktests.Test{check("[docs](https://example.com) say so", "")}},
		{"capital name is text", section("- [Backend] a"), []tasktests.Test{check("[Backend] a", "")}},
		{"one letter is text", section("- [ ] [b] a"), []tasktests.Test{check("[b] a", "")}},
		{"trailing - is text", section("- [ ] [back-] a"), []tasktests.Test{check("[back-] a", "")}},
		{"pin glued to text is text", section("- [backend]a"), []tasktests.Test{check("[backend]a", "")}},
		{"text trimmed", section("- [ ]   a  b  "), []tasktests.Test{check("a  b", "")}},

		// Kinds
		{"code span", section("- [ ] `go test ./...`"), []tasktests.Test{script("go test ./...", "go test ./...")}},
		{"pinned code span", section("- [backend] `make`"), []tasktests.Test{pinned(script("make", "make"), "backend")}},
		{"double-backtick span", section("- `` echo `date` ``"), []tasktests.Test{script("echo `date`", "echo `date`")}},
		{"two code spans", section("- `a` `b`"), []tasktests.Test{check("`a` `b`", "")}},
		{"code span in text", section("- run `make`"), []tasktests.Test{check("run `make`", "")}},
		{"code span with block", section("- `make`", "  then look"), []tasktests.Test{check("`make`", "then look")}},
		{"blank code span", section("- ` `"), []tasktests.Test{check("` `", "")}},
		{"unbalanced code span", section("- `a``"), []tasktests.Test{check("`a``", "")}},
		{"sh fence", section("- a", "  ```sh", "  echo hi", "  ```"), []tasktests.Test{script("a", "echo hi")}},
		{"bash fence", section("- a", "  ```bash", "  echo hi", "  ```"), []tasktests.Test{script("a", "echo hi")}},
		{"shell fence", section("- a", "  ```shell", "  echo hi", "  ```"), []tasktests.Test{script("a", "echo hi")}},
		{"untagged fence", section("- a", "  ```", "  echo hi", "  ```"), []tasktests.Test{script("a", "echo hi")}},
		{"tilde fence", section("- a", "  ~~~sh", "  echo hi", "  ~~~"), []tasktests.Test{script("a", "echo hi")}},
		{"fence with more info", section("- a", "  ```sh title", "  echo hi", "  ```"), []tasktests.Test{script("a", "echo hi")}},
		{"longer closing fence", section("- a", "  ```", "  echo hi", "  `````  "), []tasktests.Test{script("a", "echo hi")}},
		{"fence keeps inner indent", section("- a", "  ```sh", "  if true; then", "    echo hi", "  fi", "", "  ```"),
			[]tasktests.Test{script("a", "if true; then\n  echo hi\nfi\n")}},
		{"fence with a code span text", section("- `x`", "  "+strings.ReplaceAll(sh, "\n", "\n  ")), []tasktests.Test{script("`x`", "echo hi")}},
		{"fence naming its command", section("- make", "  ```sh", "  make", "  ```"), []tasktests.Test{script("make", "make")}},
		{"fence holding a fence", section("- a", "  ````sh", "  cat <<EOF", "  ```", "  EOF", "  ````"),
			[]tasktests.Test{script("a", "cat <<EOF\n```\nEOF")}},
		{"python fence", section("- a", "  ```python", "  print(1)", "  ```"), []tasktests.Test{check("a", "```python\nprint(1)\n```")}},
		{"fence then text", section("- a", "  ```sh", "  echo hi", "  ```", "  then look"), []tasktests.Test{check("a", sh+"\nthen look")}},
		{"text then fence", section("- a", "  first", "  ```sh", "  echo hi", "  ```"), []tasktests.Test{check("a", "first\n"+sh)}},
		{"two fences", section("- a", "  ```sh", "  echo hi", "  ```", "  ```sh", "  echo hi", "  ```"),
			[]tasktests.Test{check("a", sh+"\n"+sh)}},
		{"unclosed fence", section("- a", "  ```sh", "  echo hi"), []tasktests.Test{check("a", "```sh\necho hi")}},
		{"tilde does not close backticks", section("- a", "  ```", "  echo hi", "  ~~~"), []tasktests.Test{check("a", "```\necho hi\n~~~")}},
		{"short closing fence", section("- a", "  ````", "  echo hi", "  ```"), []tasktests.Test{check("a", "````\necho hi\n```")}},
		{"empty fence", section("- a", "  ```sh", "  ```"), []tasktests.Test{check("a", "```sh\n```")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tasktests.Parse(tc.body)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Parse = %+v, %v\nwant %+v", got, err, tc.want)
			}
			if tasktests.Validate(got) != nil {
				return // only a valid list must survive the trip
			}
			if back, err := tasktests.Parse(tasktests.Render(got)); err != nil || !reflect.DeepEqual(back, got) {
				t.Fatalf("Parse(Render) = %+v, %v\nwant %+v\nfrom\n%s", back, err, got, tasktests.Render(got))
			}
		})
	}
}

func TestParseNone(t *testing.T) {
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
		"```\n## Tests\n- a\n```",
	} {
		// Not nil, which would store null as a card's tests.
		if got, err := tasktests.Parse(body); err != nil || got == nil || len(got) != 0 {
			t.Errorf("Parse(%q) = %#v, %v; want no tests", body, got, err)
		}
	}
}

func TestParseLimit(t *testing.T) {
	var items []string
	for i := 1; i <= 64; i++ {
		items = append(items, fmt.Sprintf("- [ ] test %d", i))
	}
	got, err := tasktests.Parse(section(items...))
	if err != nil || len(got) != 64 || got[63].Name != "test 64" {
		t.Fatalf("64 tests: %d, %v", len(got), err)
	}
	_, err = tasktests.Parse(section(append(items, "- [ ] test 65")...))
	if err == nil || err.Error() != "the Tests section has 65 tests, more than 64" {
		t.Fatalf("65 tests: %v", err)
	}
}

func TestSection(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"none", "Just words.\n## Notes\n", ""},
		{"empty body", "", ""},
		{"up to the next heading", "Intro\n\n## Tests\n- a\n  one\n\n## Notes\n- b", "## Tests\n- a\n  one\n"},
		{"without trailing blank lines", "## Tests\n- a\n\n  \n", "## Tests\n- a\n"},
		{"at the end with no newline", "Intro\n## Tests\n- a", "## Tests\n- a"},
		{"CRLF kept", "## Tests\r\n- a\r\n\r\n## Notes", "## Tests\r\n- a\r\n"},
		{"with no items", "## Tests\n## Notes", "## Tests\n"},
		{"not in a fence", "```\n## Tests\n```\n## Tests\n- b", "## Tests\n- b"},
		{"through a fence", "## Tests\n- a\n```\n# x\n```\n## End", "## Tests\n- a\n```\n# x\n```\n"},
	} {
		if got := tasktests.Section(tc.body); got != tc.want {
			t.Errorf("%s: Section = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestReplace(t *testing.T) {
	tests := []tasktests.Test{check("new", "")}
	rendered := "## Tests\n- [ ] new\n"
	for _, tc := range []struct {
		name  string
		body  string
		tests []tasktests.Test
		want  string
	}{
		{"the section only", "Intro  \r\n\n## Tests\n- [x] old\n  ```sh\n  make\n  ```\n\n## Notes\nKeep  this\r\n",
			tests, "Intro  \r\n\n" + rendered + "\n## Notes\nKeep  this\r\n"},
		{"the first section only", "## Tests\n- a\n## Tests\n- b", tests, rendered + "## Tests\n- b"},
		{"appended to text", "Intro", tests, "Intro\n\n" + rendered},
		{"appended to a line", "Intro\n", tests, "Intro\n\n" + rendered},
		{"appended to a paragraph", "Intro\n\n", tests, "Intro\n\n" + rendered},
		{"appended to nothing", "", tests, rendered},
		{"removed", "Intro\n\n## Tests\n- a\n\n## Notes", nil, "Intro\n\n\n## Notes"},
		{"none to remove", "Intro\n", nil, "Intro\n"},
	} {
		got := tasktests.Replace(tc.body, tc.tests)
		if got != tc.want {
			t.Errorf("%s: Replace = %q, want %q", tc.name, got, tc.want)
		}
		if s := tasktests.Section(got); s != tasktests.Render(tc.tests) {
			t.Errorf("%s: Section after Replace = %q, want the rendered tests", tc.name, s)
		}
	}
}

func TestRender(t *testing.T) {
	tests := []tasktests.Test{
		script("make", "make"),
		pinned(script("echo `date`", "echo `date`"), "backend"),
		script("Heredoc", "cat <<EOF\n```\nEOF\n\nls"),
		check("The board loads", "1. Sign in.\n\n2. Open the board.\n   It shows five columns."),
	}
	want := "## Tests\n" +
		"- [ ] `make`\n" +
		"- [ ] [backend] `` echo `date` ``\n" +
		"- [ ] Heredoc\n  ````sh\n  cat <<EOF\n  ```\n  EOF\n\n  ls\n  ````\n" +
		"- [ ] The board loads\n  1. Sign in.\n\n  2. Open the board.\n     It shows five columns.\n"
	if got := tasktests.Render(tests); got != want {
		t.Fatalf("Render =\n%s\nwant\n%s", got, want)
	}
	if got, err := tasktests.Parse(want); err != nil || !reflect.DeepEqual(got, tests) {
		t.Fatalf("Parse(Render) = %+v, %v", got, err)
	}
	if got := tasktests.Render(nil); got != "" {
		t.Fatalf("Render(nil) = %q, want none", got)
	}
}

func TestValidate(t *testing.T) {
	bad := "Bad_Name"
	for _, tc := range []struct {
		name  string
		tests []tasktests.Test
		want  string
	}{
		{"valid", []tasktests.Test{pinned(script("make", "make"), "backend"), check("It works", "Look.")}, ""},
		{"none", []tasktests.Test{}, ""},
		{"no name", []tasktests.Test{check("ok", ""), check(" ", "")}, "Test 2 needs a name."},
		{"two lines", []tasktests.Test{check("a\nb", "")}, "Test 1's name must be one line."},
		{"a carriage return", []tasktests.Test{check("a\rb", "")}, "Test 1's name must be one line."},
		{"unknown kind", []tasktests.Test{{Name: "a", Kind: "llm"}}, `Test 1's kind must be "script" or "check".`},
		{"no command", []tasktests.Test{script("a", " \n")}, "Test 1 is a script, so it needs a command."},
		{"bad pin", []tasktests.Test{{Name: "a", Kind: "check", Workflow: &bad}},
			"Test 1's workflow must be 2 to 32 lowercase letters, digits or -, starting with a letter and ending in a letter or digit."},
		{"a NUL", []tasktests.Test{check("a", "x\x00y")}, "Test 1 holds a NUL character."},
		{"too many", make([]tasktests.Test, 65), "A task can have at most 64 tests."},
		{"too big", []tasktests.Test{check("a", strings.Repeat("x", 64<<10))}, "The tests take more than 64 KiB."},
	} {
		err := tasktests.Validate(tc.tests)
		if got := fmt.Sprint(err); tc.want == "" && err != nil || tc.want != "" && got != tc.want {
			t.Errorf("%s: Validate = %v, want %q", tc.name, err, tc.want)
		}
	}
	var most []tasktests.Test
	for i := range 64 {
		most = append(most, check(fmt.Sprint("test ", i), ""))
	}
	if err := tasktests.Validate(most); err != nil {
		t.Fatalf("64 tests: %v", err)
	}
}

// Render's fences and code spans grow past the longest backtick run in one pass, so Validate refuses a huge run at
// once, not in time that grows with its square.
func TestValidateLongBacktickRuns(t *testing.T) {
	run := strings.Repeat("`", 256<<10)
	start := time.Now()
	err := tasktests.Validate([]tasktests.Test{script("x", run), script(run, run)})
	if took := time.Since(start); fmt.Sprint(err) != "The tests take more than 64 KiB." || took > time.Second {
		t.Fatalf("Validate = %v after %v, want it refused at once", err, took)
	}
}
