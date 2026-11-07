package app_test

import (
	"testing"

	"gantt-stop-me/api/internal/apptest"
)

func TestHealth(t *testing.T) {
	res := apptest.New(t).Do("GET", "/api/health", nil)
	if res.Status != 200 || string(res.Body) != "ok" {
		t.Fatalf("GET /api/health = %d %q, want 200 \"ok\"", res.Status, res.Body)
	}
}
