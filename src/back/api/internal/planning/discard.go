package planning

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/projects"
)

// discard ends a plan in any status and drops the agent's job. Tasks it created stay.
func (p *Planning) discard(w http.ResponseWriter, r *http.Request) {
	pr, _, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	if !p.change(w, r, pr.ID, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		_, err := tx.Exec(ctx, `update feature_plans set status = 'discarded', pending = null where id = $1`, pl.ID)
		return err
	}) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
