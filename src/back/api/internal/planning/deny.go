package planning

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// deny sends a plan's doc back to briefing with the reviewer's feedback, for the agent to ask about: the sections
// it points at go back to not_yet.
func (p *Planning) deny(w http.ResponseWriter, r *http.Request) {
	pr, _, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Feedback string   `json:"feedback"`
		Sections []string `json:"sections"` // keys
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	feedback, ok := text(in.Feedback, maxText)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_feedback", "Feedback needs 1 to 10000 characters.")
		return
	}
	var reopen []change
	for _, key := range in.Sections {
		reopen = append(reopen, change{Key: key, State: "not_yet"})
	}
	if _, err := apply(topics, reopen); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_sections", "In sections, "+err.Error()+".")
		return
	}
	if !p.change(w, r, pr.ID, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		if err := waiting(pl, "needs_review"); err != nil {
			return err
		}
		doc, err := apply(pl.Doc, reopen)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update feature_plans set status = 'briefing', pending = 'ask', feedback = $2, doc = $3 where id = $1`,
			pl.ID, feedback, doc)
		return err
	}) {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
