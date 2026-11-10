package planning

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// denyProposal turns down a proposal waiting for review with the reviewer's feedback: revise sends it back, for the
// agent to rewrite it with the feedback, and drop drops it. Once no proposal waits, the plan is done. A proposal sent
// back whose revision failed can be denied again, so a revision that keeps failing is not a dead end.
func (p *Planning) denyProposal(w http.ResponseWriter, r *http.Request) {
	pr, token, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Feedback string `json:"feedback"`
		Outcome  string `json:"outcome"` // "revise" or "drop"
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	feedback, ok := text(in.Feedback, maxText)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_feedback", "Feedback needs 1 to 10000 characters.")
		return
	}
	status := map[string]string{"revise": "sent_back", "drop": "dropped"}[in.Outcome]
	if status == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_outcome", "The outcome needs to be revise or drop.")
		return
	}
	if !p.change(w, r, pr.ID, token, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		pp, err := reviewing(ctx, tx, r, pl, "needs_review", "sent_back")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `update plan_proposals set status = $2, feedback = $3 where id = $1`, pp.ID, status, feedback); err != nil {
			return err
		}
		if status == "sent_back" {
			_, err = tx.Exec(ctx, `update feature_plans set pending = 'revise' where id = $1`, pl.ID)
			return err
		}
		return settle(ctx, tx, pl.ID)
	}) {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
