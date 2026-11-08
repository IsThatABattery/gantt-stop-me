package automation_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/runner/runnertest"
)

func TestVideo(t *testing.T) {
	f := withLLM(t)
	f.tests(1, "## Tests\n- [ ] `true`\n- [ ] The sign-up page refuses an empty email\n")
	f.repo.AddFile("a1", ".gantt/workflows.yml", withApp)
	f.llm.Reply(call("browser_navigate", `{"url": "http://app:80/signup"}`), reported("pass", "It refused."))
	f.ready()
	f.work()
	f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "") // it stays readable once off

	ids := f.resultIDs()
	if len(ids) != 2 {
		t.Fatalf("results %v", ids)
	}
	run := f.lastRun()
	reader, _, _ := f.member("bob", "pull")
	var shown shownRun
	f.Do("GET", f.path(fmt.Sprintf("/runs/%d", run)), nil, reader).JSON(&shown)
	if len(shown.Results) != 2 || shown.Results[0].HasVideo || !shown.Results[1].HasVideo {
		t.Fatalf("results = %+v, want hasVideo on the LLM check alone", shown.Results)
	}
	video := func(run, result int64) string {
		return f.path(fmt.Sprintf("/runs/%d/results/%d/video", run, result))
	}
	res := f.Do("GET", video(run, ids[1]), nil, reader)
	res.Expect(http.StatusOK, "")
	if res.Header.Get("Content-Type") != "video/webm" || res.Header.Get("Accept-Ranges") != "bytes" || !bytes.Equal(res.Body, runnertest.WebM) {
		t.Fatalf("got %s, ranges %q, with %d bytes; want the WebM", res.Header.Get("Content-Type"), res.Header.Get("Accept-Ranges"), len(res.Body))
	}

	// A player seeks with a Range request.
	req, err := http.NewRequest("GET", f.Server.URL+video(run, ids[1]), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=4-")
	req.AddCookie(reader)
	ranged, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer ranged.Body.Close()
	body, _ := io.ReadAll(ranged.Body)
	if ranged.StatusCode != http.StatusPartialContent || !bytes.Equal(body, runnertest.WebM[4:]) {
		t.Fatalf("a Range request got %d %q, want 206 with the rest of the WebM", ranged.StatusCode, body)
	}

	f.Do("GET", video(run, ids[0]), nil, reader).Expect(http.StatusNotFound, "not_found")   // a script has none
	f.Do("GET", video(run+1, ids[1]), nil, reader).Expect(http.StatusNotFound, "not_found") // another run's
	stranger := f.SignUp("eve@example.com", "correct horse")
	f.LinkGitHub(stranger, "eve")
	f.Do("GET", video(run, ids[1]), nil, stranger).Expect(http.StatusNotFound, "not_found")
}
