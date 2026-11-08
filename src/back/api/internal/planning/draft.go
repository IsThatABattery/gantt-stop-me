package planning

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/projects"
)

// draft ends the interview early: the agent's job becomes writing the doc from what it has.
func (p *Planning) draft(w http.ResponseWriter, r *http.Request) {
	pr, _, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	if !p.change(w, r, pr.ID, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		if err := waiting(pl, "briefing"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `update feature_plans set pending = 'draft' where id = $1`, pl.ID)
		return err
	}) {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
