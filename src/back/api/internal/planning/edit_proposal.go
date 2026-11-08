package planning

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/tasktests"
)

// editProposal changes a proposal waiting for review as a person wrote it, unless the plan changed since they loaded
// its version. Approving it then records it as approved_edited.
func (p *Planning) editProposal(w http.ResponseWriter, r *http.Request) {
	pr, _, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct { // nil keeps a field
		Title              *string          `json:"title"`
		Description        *string          `json:"description"`
		AcceptanceCriteria []string         `json:"acceptanceCriteria"`
		Tests              []tasktests.Test `json:"tests"`
		DependsOn          []int            `json:"dependsOn"`
		Version            int              `json:"version"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if !p.change(w, r, pr.ID, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		pp, err := reviewing(ctx, tx, r, pl, "needs_review")
		if err != nil {
			return err
		}
		if in.Version != pl.Version {
			return errConflict
		}
		if in.Title != nil {
			pp.Title = *in.Title
		}
		if in.Description != nil {
			pp.Description = *in.Description
		}
		if in.AcceptanceCriteria != nil {
			pp.AcceptanceCriteria = in.AcceptanceCriteria
		}
		if in.Tests != nil {
			pp.Tests = in.Tests
		}
		if in.DependsOn != nil {
			pp.DependsOn = in.DependsOn
		}
		pp.Edited = true
		if err := tidy(&pp); err != nil {
			return err
		}
		if _, err := body(ctx, tx, pl.ID, pp); err != nil {
			return err
		}
		return write(ctx, tx, pl.ID, pp)
	}) {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
