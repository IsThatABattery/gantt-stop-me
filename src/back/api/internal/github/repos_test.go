package github_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"gantt-stop-me/api/internal/github"
)

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
