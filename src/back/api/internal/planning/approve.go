package planning

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/projects"
)

// approve accepts a plan's doc, which makes generating its tasks the agent's job.
func (p *Planning) approve(w http.ResponseWriter, r *http.Request) {
	pr, token, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	if !p.change(w, r, pr.ID, token, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		if err := waiting(pl, "needs_review"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `update feature_plans set status = 'approved', pending = 'generate' where id = $1`, pl.ID)
		return err
	}) {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
