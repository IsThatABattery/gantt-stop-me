package github_test

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/github/githubtest"
)

func TestIssue(t *testing.T) {
	_, repo, gh := fake(t)
	n := repo.AddIssue("Sign up")
	repo.EditIssue(n, func(is *githubtest.Issue) { is.Body = "## Tests\n- [ ] `go test ./...`" })
	is, err := gh.Issue(t.Context(), "token", repo.ID, n)
	if err != nil || is.Title != "Sign up" || is.Body != "## Tests\n- [ ] `go test ./...`" || is.State != "open" {
		t.Fatalf("Issue = %+v, %v", is, err)
	}
	if _, err := gh.Issue(t.Context(), "token", repo.ID, 99); !isStatus(err, http.StatusNotFound) {
		t.Fatalf("Issue of a missing issue: err = %v, want HTTP 404", err)
	}
}

func TestWorkflowFile(t *testing.T) {
	f, repo, gh := fake(t)
	repo.AddFile("a1", ".gantt/workflows.yml", "workflows: {}\n")
	repo.AddFile("b2", ".gantt/workflows.yml", strings.Repeat("#", 100<<10))
	// The fake answers JSON unless asked for the raw file, so this also shows the client asks for it.
	if data, err := gh.WorkflowFile(t.Context(), "token", repo.ID, "a1"); err != nil || string(data) != "workflows: {}\n" {
		t.Fatalf("WorkflowFile = %q, %v", data, err)
	}
	if want := fmt.Sprintf("GET /repositories/%d/contents/.gantt/workflows.yml?ref=a1", repo.ID); !slices.Contains(f.Requests(), want) {
		t.Errorf("requests = %v, want %s", f.Requests(), want)
	}
	if data, err := gh.WorkflowFile(t.Context(), "token", repo.ID, "b2"); err != nil || len(data) != 64<<10+1 {
		t.Fatalf("WorkflowFile of 100 KiB = %d bytes, %v; want 64 KiB and one", len(data), err)
	}
	if _, err := gh.WorkflowFile(t.Context(), "token", repo.ID, "c3"); !isStatus(err, http.StatusNotFound) {
		t.Fatalf("WorkflowFile at a commit without one: err = %v, want HTTP 404", err)
	}
	f.Fail("GET /repositories/{id}/contents/{path...}", http.StatusBadGateway)
	if _, err := gh.WorkflowFile(t.Context(), "token", repo.ID, "a1"); !isStatus(err, http.StatusBadGateway) {
		t.Fatalf("WorkflowFile with GitHub failing: err = %v", err)
	}
}

func TestTarball(t *testing.T) {
	f, repo, gh := fake(t)
	repo.AddFile("0123456789", "Dockerfile", "FROM scratch\n")
	repo.AddFile("0123456789", "src/main.go", "package main\n")
	repo.AddFile("other", "README", "not in this commit")
	body, err := gh.Tarball(t.Context(), "token", repo.ID, "0123456789")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	gz, err := gzip.NewReader(body)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var got []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(tr)
		got = append(got, fmt.Sprintf("%c %s %s %s", h.Typeflag, h.Name, h.PAXRecords["comment"], data))
	}
	want := []string{
		"g pax_global_header 0123456789 ",
		"5 octo-hello-0123456/  ",
		"0 octo-hello-0123456/Dockerfile  FROM scratch\n",
		"0 octo-hello-0123456/src/main.go  package main\n",
	}
	if !slices.Equal(got, want) {
		t.Errorf("tarball entries = %q, want %q", got, want)
	}
	// codeload refuses a request carrying a token, so getting here shows the token stayed on the API.
	if want := "GET /octo/hello/legacy.tar.gz/0123456789"; !slices.Contains(f.Requests(), want) {
		t.Errorf("requests = %v, want the redirect to %s", f.Requests(), want)
	}

	f.Fail("GET /repositories/{id}/tarball/{sha}", http.StatusNotFound)
	if _, err := gh.Tarball(t.Context(), "token", repo.ID, "0123456789"); !isStatus(err, http.StatusNotFound) {
		t.Fatalf("Tarball with GitHub failing: err = %v, want HTTP 404", err)
	}
}

// isStatus reports whether err is GitHub answering status.
func isStatus(err error, status int) bool {
	ghErr := (*github.Error)(nil)
	return errors.As(err, &ghErr) && ghErr.Status == status
}

func TestIssuesRefusesNextPageOffAPI(t *testing.T) {
	var otherHits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherHits.Add(1)
		w.Write([]byte("[]"))
	}))
	defer other.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<`+other.URL+`/repositories/1/issues?page=2>; rel="next"`)
		w.Write([]byte("[]"))
	}))
	defer api.Close()

	gh := &github.Client{API: api.URL, HTTP: api.Client()}
	_, err := gh.Issues(t.Context(), "secret", 1, nil, "", func([]github.Issue) bool { return true })
	if err == nil {
		t.Fatal("Issues followed a next page off the API host")
	}
	if n := otherHits.Load(); n != 0 {
		t.Fatalf("the other host got %d requests carrying the token", n)
	}
}
