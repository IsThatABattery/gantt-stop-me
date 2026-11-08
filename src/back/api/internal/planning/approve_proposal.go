package planning

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/projects"
)

// approveProposal approves a proposal waiting for review and creates its task with the approver's GitHub token, at
// the end of Backlog. Once no proposal waits, the plan is done.
func (p *Planning) approveProposal(w http.ResponseWriter, r *http.Request) {
	pr, token, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	// Once GitHub has the issue, record its task even if the browser goes away.
	r = r.WithContext(context.WithoutCancel(r.Context()))
	if !p.change(w, r, pr.ID, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		pp, err := reviewing(ctx, tx, r, pl)
		if err != nil {
			return err
		}
		if err := p.createTask(ctx, tx, token, pr, pl.ID, pp); err != nil {
			return err
		}
		return settle(ctx, tx, pl.ID)
	}) {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
