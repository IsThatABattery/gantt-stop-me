package planning

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/projects"
)

// retry gives the agent back the job its last turn failed: the draft when a question waits unanswered without
// feedback, since only a draft starts from there, else the next question.
// shortcut: a draft asked for after a failed question, which fails too, is retried as a question; store the failed
// job if that confuses people.
func (p *Planning) retry(w http.ResponseWriter, r *http.Request) {
	pr, _, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	if !p.change(w, r, pr.ID, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		if err := waiting(pl, "briefing"); err != nil {
			return err
		}
		if pl.Error == nil {
			return &refusal{http.StatusConflict, "nothing_to_retry", "The agent's last turn did not fail."}
		}
		_, err := tx.Exec(ctx, `
			update feature_plans fp set pending = case when fp.feedback is null and
				(select role from plan_messages where plan_id = fp.id order by id desc limit 1) = 'agent' then 'draft' else 'ask' end
			where id = $1`, pl.ID)
		return err
	}) {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
