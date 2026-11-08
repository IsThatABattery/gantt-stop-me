package board_test

import (
	"cmp"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/tasktests"
)

// check is a check named name with details.
func check(name, details string) tasktests.Test {
	return tasktests.Test{Name: name, Kind: "check", Details: details}
}

// script is a script named by its command.
func script(command string) tasktests.Test {
	return tasktests.Test{Name: command, Kind: "script", Command: command}
}

// setTests puts tests on the card of issue number as session.
func (f *fixture) setTests(session *http.Cookie, number int, tests ...tasktests.Test) *apptest.Response {
	f.t.Helper()
	id, _ := f.task(number)
	return f.Do("PUT", f.path("/tasks/%d/tests", id), map[string]any{"tests": append([]tasktests.Test{}, tests...)}, session)
}

// expectCard fails the test unless GET task answers ada the card of issue number with tests and testsError, "" for
// none.
func (f *fixture) expectCard(number int, testsError string, tests ...tasktests.Test) {
	f.t.Helper()
	id, _ := f.task(number)
	res := f.Do("GET", f.path("/tasks/%d", id), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var got struct {
		Tests      []tasktests.Test
		TestsError *string
	}
	res.JSON(&got)
	if want := append([]tasktests.Test{}, tests...); !reflect.DeepEqual(got.Tests, want) || orNull(got.TestsError) != cmp.Or(testsError, "null") {
		f.t.Fatalf("card = %+v, error %s; want %+v, error %q", got.Tests, orNull(got.TestsError), want, testsError)
	}
}

// tooMany is a Tests section with 65 items, which does not parse.
var tooMany = "## Tests\n" + strings.Repeat("- [ ] `true`\n", 65)

func TestSetTests(t *testing.T) {
	f := setup(t, "One")
	before, after := "Intro  \r\nwith *CRLF*\r\n\r\n", "\r\n## Notes\nKeep\tthis.\n"
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = before + "## Tests\r\n- [x] `old`\r\n" + after })
	f.dueSync()
	f.expectCard(1, "", script("old"))

	tests := []tasktests.Test{script("go test ./..."), check("The board loads", "1. Open it.\n2. Look.")}
	res := f.setTests(f.ada, 1, tests...)
	res.Expect(http.StatusOK, "")
	var got struct{ Tests []tasktests.Test }
	res.JSON(&got)
	if !reflect.DeepEqual(got.Tests, tests) {
		t.Fatalf("answered %+v, want %+v", got.Tests, tests)
	}
	// Byte for byte outside the section.
	want := before + "## Tests\n- [ ] `go test ./...`\n- [ ] The board loads\n  1. Open it.\n  2. Look.\n" + after
	if is := f.repo.Issue(1); is.Body != want {
		t.Fatalf("issue body = %q, want %q", is.Body, want)
	}
	f.expectCard(1, "", tests...)
	if body, _ := f.details(1); body != want {
		t.Fatalf("stored body = %q, want the issue's", body)
	}
	f.dueSync() // finds the section the card wrote
	f.expectCard(1, "", tests...)

	f.setTests(f.ada, 1).Expect(http.StatusOK, "")
	if is := f.repo.Issue(1); is.Body != before+after {
		t.Fatalf("issue body = %q, want the section removed", is.Body)
	}
	f.expectCard(1, "")
}

func TestSetTestsAppendsTheSection(t *testing.T) {
	f := setup(t, "One")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "Draw it." })
	f.dueSync()
	f.setTests(f.ada, 1, tasktests.Test{Name: "Builds", Kind: "script", Command: "make\nmake check"}).Expect(http.StatusOK, "")
	if is := f.repo.Issue(1); is.Body != "Draw it.\n\n## Tests\n- [ ] Builds\n  ```sh\n  make\n  make check\n  ```\n" {
		t.Fatalf("issue body = %q", is.Body)
	}
}

// The card stores the tests as the issue reads back, so the two never disagree.
func TestSetTestsAsGitHubReadsThem(t *testing.T) {
	f := setup(t, "One")
	backend := "backend"
	res := f.setTests(f.ada, 1, check("[backend] `make`", ""), tasktests.Test{Name: "  Lints ", Kind: "check", Command: "ignored"})
	res.Expect(http.StatusOK, "")
	want := []tasktests.Test{{Name: "make", Kind: "script", Command: "make", Workflow: &backend}, check("Lints", "")}
	var got struct{ Tests []tasktests.Test }
	res.JSON(&got)
	if !reflect.DeepEqual(got.Tests, want) {
		t.Fatalf("answered %+v, want %+v", got.Tests, want)
	}
	f.expectCard(1, "", want...)
}

func TestSetTestsWhenGitHubChangedFirst(t *testing.T) {
	f := setup(t, "One")
	f.setTests(f.ada, 1, check("Old", "")).Expect(http.StatusOK, "")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "## Tests\n- [ ] `make`\n" }) // no sync since

	res := f.setTests(f.ada, 1, check("Mine", ""))
	res.Expect(http.StatusConflict, "tests_changed")
	var got struct {
		Tests      []tasktests.Test
		TestsError *string
	}
	res.JSON(&got)
	if !reflect.DeepEqual(got.Tests, []tasktests.Test{script("make")}) || got.TestsError != nil {
		t.Fatalf("answered %+v, error %v; want GitHub's", got.Tests, orNull(got.TestsError))
	}
	if is := f.repo.Issue(1); is.Body != "## Tests\n- [ ] `make`\n" {
		t.Fatalf("issue body = %q, want GitHub's kept", is.Body)
	}
	f.expectCard(1, "", script("make"))

	// The card has seen GitHub's version now, so the same edit goes through.
	f.setTests(f.ada, 1, check("Mine", "")).Expect(http.StatusOK, "")
	// A change on GitHub that leaves the tests as they are, such as a ticked box, loses nothing.
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = "## Tests\n- [x] Mine\n" })
	f.setTests(f.ada, 1, check("Mine", ""), check("Two", "")).Expect(http.StatusOK, "")
	if is := f.repo.Issue(1); is.Body != "## Tests\n- [ ] Mine\n- [ ] Two\n" {
		t.Fatalf("issue body = %q", is.Body)
	}
}

// A section that does not parse can never be taken, so after one 409 that shows why, a card edit replaces it.
func TestSetTestsOverASectionThatDoesNotParse(t *testing.T) {
	f := setup(t, "One")
	f.setTests(f.ada, 1, check("Old", "")).Expect(http.StatusOK, "")
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = tooMany })

	res := f.setTests(f.ada, 1, check("Mine", ""))
	res.Expect(http.StatusConflict, "tests_changed")
	var got struct {
		Tests      []tasktests.Test
		TestsError *string
	}
	res.JSON(&got)
	if !reflect.DeepEqual(got.Tests, []tasktests.Test{check("Old", "")}) || orNull(got.TestsError) != "the Tests section has 65 tests, more than 64" {
		t.Fatalf("answered %+v, error %v; want the card kept and why", got.Tests, orNull(got.TestsError))
	}
	f.setTests(f.ada, 1, check("Mine", "")).Expect(http.StatusOK, "")
	if is := f.repo.Issue(1); is.Body != "## Tests\n- [ ] Mine\n" {
		t.Fatalf("issue body = %q", is.Body)
	}
	f.expectCard(1, "", check("Mine", ""))
}

func TestSetTestsValidates(t *testing.T) {
	f := setup(t, "One")
	id, _ := f.task(1)
	res := f.setTests(f.ada, 1, check("ok", ""), check(" ", ""))
	res.Expect(http.StatusBadRequest, "invalid_tests")
	var e struct{ Error struct{ Message string } }
	res.JSON(&e)
	if e.Error.Message != "Test 2 needs a name." {
		t.Fatalf("message = %q", e.Error.Message)
	}
	f.Do("PUT", f.path("/tasks/%d/tests", id), "{", f.ada).Expect(http.StatusBadRequest, "bad_request")
	f.Do("PUT", f.path("/tasks/%d/tests", id), map[string]any{}, f.ada).Expect(http.StatusBadRequest, "invalid_tests")
	f.Do("PUT", f.path("/tasks/999/tests"), map[string]any{"tests": []any{}}, f.ada).Expect(http.StatusNotFound, "not_found")

	// Over GitHub's 65536 characters once in the body.
	long := strings.Repeat("é", 65530)
	f.repo.EditIssue(1, func(is *githubtest.Issue) { is.Body = long })
	f.setTests(f.ada, 1, check("A name that does not fit", "")).Expect(http.StatusBadRequest, "invalid_body")
	if is := f.repo.Issue(1); is.Body != long {
		t.Fatal("the issue changed")
	}
	f.expectCard(1, "")
}

func TestSetTestsNeedsWrite(t *testing.T) {
	f := setup(t, "One")
	f.setTests(f.reader(), 1, check("Nope", "")).Expect(http.StatusForbidden, "forbidden")
	f.setTests(nil, 1, check("Nope", "")).Expect(http.StatusUnauthorized, "unauthorized")
	if is := f.repo.Issue(1); is.Body != "" {
		t.Fatalf("issue body = %q, want it unchanged", is.Body)
	}
}

func TestSetTestsWhenGitHubFails(t *testing.T) {
	for _, route := range []string{"GET /repositories/{id}/issues/{number}", "PATCH /repositories/{id}/issues/{number}"} {
		t.Run(route, func(t *testing.T) {
			f := setup(t, "One")
			f.setTests(f.ada, 1, check("Old", "")).Expect(http.StatusOK, "")
			f.GitHub.Fail(route, http.StatusInternalServerError)
			f.setTests(f.ada, 1, check("Lost", "")).Expect(http.StatusBadGateway, "github_error")
			f.GitHub.Fail(route, 0)
			f.expectCard(1, "", check("Old", ""))
			if is := f.repo.Issue(1); is.Body != "## Tests\n- [ ] Old\n" {
				t.Fatalf("issue body = %q", is.Body)
			}
		})
	}
}
