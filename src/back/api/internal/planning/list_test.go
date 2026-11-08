package planning_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	f := setup(t)
	if res := f.Do("GET", f.path("/plans"), nil, f.ada); strings.TrimSpace(string(res.Body)) != `{"plans":[]}` {
		t.Fatalf("empty list = %s", res.Body)
	}
	first := f.toReview()
	second := f.start("Imports", "Import a CSV.")

	res := f.Do("GET", f.path("/plans"), nil, f.member("bob", "pull"))
	res.Expect(http.StatusOK, "")
	var body struct{ Plans []planJSON }
	res.JSON(&body)
	if len(body.Plans) != 2 || body.Plans[0].ID != second.ID || body.Plans[1].ID != first.ID {
		t.Fatalf("plans %+v, want newest first", body.Plans)
	}
	if p := body.Plans[1]; p.Status != "needs_review" || p.Title != "Exports" || len(p.Doc) != 8 || p.ExpectedQuestions != 0 ||
		p.ProjectID != f.project {
		t.Fatalf("listed plan %+v", p)
	}
	if strings.Contains(string(res.Body), `"messages"`) {
		t.Fatalf("list %s has messages", res.Body)
	}
}
