package planning

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/llm"
)

// turnTimeout bounds one agent turn, which may make many LLM calls.
const turnTimeout = 5 * time.Minute

// Work runs the planner worker once, as Run does every 2 seconds: one interviewer turn for each plan whose job is a
// question or the draft, in id order. Generating tasks waits, since no agent takes that job yet. Tests call Work in
// place of Run, which never runs there.
func (p *Planning) Work(ctx context.Context) {
	for after := int64(0); ctx.Err() == nil; {
		id, err := p.turn(ctx, after)
		if err != nil && ctx.Err() == nil {
			log.Printf("planner turn after plan %d: %v", after, err)
		}
		if id == 0 {
			return
		}
		after = id
	}
}

// Run runs the planner worker every 2 seconds until ctx ends. A turn that ctx ends keeps its job for the next start.
// shortcut: turns run one after another, so a slow provider delays every plan's turn, by up to turnTimeout each;
// give each its own goroutine if that bites.
func (p *Planning) Run(ctx context.Context) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		p.Work(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// turn takes the first plan after id after whose job is a question or the draft, holding its row so requests answer
// agent_busy meanwhile, runs the interviewer on it with its creator's planning model and saves what it did. It returns
// the plan's id, or 0 when there is none. A failed turn records why and drops the job, for a person to retry.
func (p *Planning) turn(ctx context.Context, after int64) (int64, error) {
	tx, err := p.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	pl, err := scanPlan(tx.QueryRow(ctx, `select `+planColumns+` from feature_plans fp
		where fp.pending in ('ask', 'draft') and fp.id > $1 order by fp.id limit 1 for update of fp skip locked`, after))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	msgs, err := messages(ctx, tx, pl.ID)
	if err != nil {
		return pl.ID, err
	}
	var repo, baseURL, model string
	var sealed []byte
	var review bool
	err = tx.QueryRow(ctx, `
		select pr.owner || '/' || pr.name, pv.base_url, pv.key_enc, ps.model, ps.review
		from feature_plans fp join projects pr on pr.id = fp.project_id
		join planning_settings ps on ps.user_id = fp.created_by join providers pv on pv.id = ps.provider_id
		where fp.id = $1`, pl.ID).Scan(&repo, &baseURL, &sealed, &model, &review)
	var failure string // why the turn failed, for people to read
	var doc []section
	var question *message
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		failure = "The plan's creator has no planning model. They pick one in Settings, then someone retries."
	case err != nil:
		return pl.ID, err
	default:
		key, err := p.Secrets.Open(sealed)
		if err != nil {
			return pl.ID, err
		}
		c := &llm.Client{BaseURL: baseURL, Key: string(key), HTTP: p.LLM}
		tctx, cancel := context.WithTimeout(ctx, turnTimeout)
		doc, question, err = interview(tctx, c, model, repo, pl, msgs)
		cancel()
		if err != nil {
			failure = "The agent's turn failed: " + strings.TrimPrefix(err.Error(), "llm: ") + "."
		}
	}
	if ctx.Err() != nil { // shutting down: the job stays for the next start
		return pl.ID, ctx.Err()
	}

	status, pending := "briefing", (*string)(nil)
	switch {
	case failure != "":
		_, err = tx.Exec(ctx, `update feature_plans set pending = null, error = $2, version = version + 1, updated_at = now()
			where id = $1`, pl.ID, failure)
		if err != nil {
			return pl.ID, err
		}
		return pl.ID, tx.Commit(ctx)
	case question != nil:
		_, err = tx.Exec(ctx, `insert into plan_messages (plan_id, role, text, options) values ($1, 'agent', $2, $3)`,
			pl.ID, question.Text, question.Options)
		if err != nil {
			return pl.ID, err
		}
	case review:
		status = "needs_review"
	default:
		status, pending = "approved", new("generate")
	}
	_, err = tx.Exec(ctx, `update feature_plans set doc = $2, status = $3, pending = $4, error = null, feedback = null,
		version = version + 1, updated_at = now() where id = $1`, pl.ID, doc, status, pending)
	if err != nil {
		return pl.ID, err
	}
	return pl.ID, tx.Commit(ctx)
}
