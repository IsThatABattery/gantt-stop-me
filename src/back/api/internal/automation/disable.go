package automation

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// disable turns automation off: it forgets the project's PR records, cancels its queued and running test runs, whose
// running workflow executions the queue worker then stops, and skips their unfinished tests. It holds the project's
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
		// A finished run's comment not posted yet never is: no owner is left to post it, and turning automation on
		// again later would post a stale one.
		rows, _ := tx.Query(ctx, `
			with off as (delete from project_automation where project_id = $1),
			records as (delete from pull_requests where project_id = $1),
			unposted as (
				update test_runs set commented_at = now() where commented_at is null and status in ('passed', 'failed', 'error')
					and task_id in (select id from tasks where project_id = $1))
			update test_runs set status = 'cancelled', reason = 'Automation was turned off.', finished_at = now()
			where status in ('queued', 'running') and task_id in (select id from tasks where project_id = $1)
			returning id`, p.ID)
		ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
		if err != nil || len(ids) == 0 {
			return err
		}
		return skipRest(ctx, tx, ids)
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
