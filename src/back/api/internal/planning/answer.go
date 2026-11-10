package planning

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// answer answers the agent's last question, or skips it, and makes its next question the agent's job.
func (p *Planning) answer(w http.ResponseWriter, r *http.Request) {
	pr, token, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Text string `json:"text"`
		Skip bool   `json:"skip"` // the text is ignored
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	answer := ""
	if !in.Skip {
		if answer, ok = text(in.Text, maxText); !ok {
			httpx.Error(w, http.StatusBadRequest, "invalid_text", "An answer needs 1 to 10000 characters, or skip it.")
			return
		}
	}
	if !p.change(w, r, pr.ID, token, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		if err := waiting(pl, "briefing"); err != nil {
			return err
		}
		var role string
		err := tx.QueryRow(ctx, `select role from plan_messages where plan_id = $1 order by id desc limit 1`, pl.ID).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && role != "agent" {
			// After a failed turn, which only a retry or a draft takes on from.
			return &refusal{http.StatusConflict, "no_question", "No question waits for an answer. Retry the agent's turn instead."}
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `insert into plan_messages (plan_id, role, text, skipped) values ($1, 'user', $2, $3)`,
			pl.ID, answer, in.Skip); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update feature_plans set pending = 'ask' where id = $1`, pl.ID)
		return err
	}) {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
