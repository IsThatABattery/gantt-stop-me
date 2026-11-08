package automation

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// disable turns automation off: it forgets the project's PR records, cancels its queued and running test runs and
// workflow executions, which the queue worker then stops, and skips their unfinished tests. It holds the project's
// lock, so no trigger saves a PR record or starts a run halfway through. Already off is not an error.
func (a *Automation) disable(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	ctx := r.Context()
	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return // the client gave up waiting
	}
	defer unlock()
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			with off as (delete from project_automation where project_id = $1),
			records as (delete from pull_requests where project_id = $1),
			runs as (
				update test_runs set status = 'cancelled', reason = 'Automation was turned off.', finished_at = now()
				where status in ('queued', 'running') and task_id in (select id from tasks where project_id = $1)
				returning id),
			results as (update test_results set status = 'skipped' where status = 'pending' and run_id in (select id from runs))
			update workflow_executions set status = 'cancelled', finished_at = now()
			where project_id = $1 and status in ('queued', 'running')`, p.ID)
		return err
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
