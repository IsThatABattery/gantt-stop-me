package automation_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/runner/runnertest"
)

func TestScreenshot(t *testing.T) {
	f := withLLM(t)
	f.tests(1, "## Tests\n- [ ] `true`\n- [ ] The sign-up page refuses an empty email\n")
	f.repo.AddFile("a1", ".gantt/workflows.yml", withApp)
	f.llm.Reply(call("browser_navigate", `{"url": "http://app:80/signup"}`), reported("pass", "It refused."))
	f.ready()
	f.work()
	f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "") // it stays readable once off

	rows, _ := f.DB.Query(context.Background(), `select id from test_results order by position`)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil || len(ids) != 2 {
		t.Fatalf("results %v, %v", ids, err)
	}
	run := f.lastRun()
	screenshot := func(run, result int64) string {
		return f.path(fmt.Sprintf("/runs/%d/results/%d/screenshot", run, result))
	}
	reader, _, _ := f.member("bob", "pull")
	res := f.Do("GET", screenshot(run, ids[1]), nil, reader)
	res.Expect(http.StatusOK, "")
	if res.Header.Get("Content-Type") != "image/png" || !bytes.Equal(res.Body, runnertest.PNG) {
		t.Fatalf("got %s with %d bytes, want the PNG", res.Header.Get("Content-Type"), len(res.Body))
	}
	f.Do("GET", screenshot(run, ids[0]), nil, reader).Expect(http.StatusNotFound, "not_found")   // a script has none
	f.Do("GET", screenshot(run+1, ids[1]), nil, reader).Expect(http.StatusNotFound, "not_found") // another run's
	stranger := f.SignUp("eve@example.com", "correct horse")
	f.LinkGitHub(stranger, "eve")
	f.Do("GET", screenshot(run, ids[1]), nil, stranger).Expect(http.StatusNotFound, "not_found")
}
