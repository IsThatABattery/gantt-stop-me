package planning

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/projects"
)

// turnTimeout bounds one agent turn, which may make many LLM calls.
const turnTimeout = 5 * time.Minute

// Work runs the planner worker once, as Run does every 2 seconds, and waits for the turns it starts. Tests call Work
// in place of Run, which never runs there.
func (p *Planning) Work(ctx context.Context) {
	var turns sync.WaitGroup
	p.startAll(ctx, &turns)
	turns.Wait()
}

// Run runs the planner worker every 2 seconds until ctx ends, then waits for its turns. A turn that ctx ends keeps
// its job for the next start.
func (p *Planning) Run(ctx context.Context) {
	var turns sync.WaitGroup
	defer turns.Wait()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		p.startAll(ctx, &turns)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// startAll starts an agent turn, in a goroutine of turns, for each plan with a job, in id order. A plan whose
// creator has a turn running waits for a later pass, so a slow provider delays only its own user's plans, and turns
// hold at most half the database pool, one connection each and a second briefly.
// shortcut: a turn creating tasks lets go of its plan's row between them, and only running, in this process, keeps
// another turn off the plan meanwhile; claim jobs outside the pool before running more than one api instance.
// shortcut: that many creators whose providers hang still hold up everyone else's turns, by up to turnTimeout each;
// free the connection during LLM calls if that bites.
func (p *Planning) startAll(ctx context.Context, turns *sync.WaitGroup) {
	for after := int64(0); ctx.Err() == nil; {
		p.mu.Lock()
		busy := []int64{} // not nil, which the claim would read as NULL and match nothing
		for creator := range p.running {
			busy = append(busy, creator)
		}
		p.mu.Unlock()
		if len(busy) >= max(1, int(p.DB.Config().MaxConns)/2) {
			return
		}
		tx, pl, err := p.claim(ctx, after, busy)
		if err != nil && ctx.Err() == nil {
			log.Printf("planner: claim a plan after %d: %v", after, err)
		}
		if tx == nil {
			return
		}
		after = pl.ID
		p.mu.Lock()
		if p.running == nil {
			p.running = map[int64]bool{}
		}
		p.running[pl.createdBy] = true
		p.mu.Unlock()
		turns.Go(func() {
			if err := p.turn(ctx, tx, pl); err != nil && ctx.Err() == nil {
				log.Printf("planner turn on plan %d: %v", pl.ID, err)
			}
			p.mu.Lock()
			delete(p.running, pl.createdBy)
			p.mu.Unlock()
		})
	}
}

// claim takes the first plan after id after with a job whose creator is not busy, in a transaction holding its row,
// so requests answer agent_busy meanwhile. It returns a nil tx when there is none.
func (p *Planning) claim(ctx context.Context, after int64, busy []int64) (pgx.Tx, plan, error) {
	tx, err := p.DB.Begin(ctx)
	if err != nil {
		return nil, plan{}, err
	}
	pl, err := scanPlan(tx.QueryRow(ctx, `select `+planColumns+` from feature_plans fp
		where fp.pending is not null and fp.id > $1 and coalesce(fp.created_by, 0) <> all($2)
		order by fp.id limit 1 for update of fp skip locked`, after, busy))
	if err != nil {
		tx.Rollback(context.WithoutCancel(ctx))
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
		return nil, plan{}, err
	}
	return tx, pl, nil
}

// turn runs the agent of plan pl's job on it, whose row tx holds, with its creator's model and instructions for that
// agent, and saves what it did in tx: the interviewer for a question or the draft, else the task agent. A failed turn records why and drops
// the job, for a person to retry.
func (p *Planning) turn(ctx context.Context, tx pgx.Tx, pl plan) error {
	defer tx.Rollback(context.WithoutCancel(ctx))
	msgs, err := messages(ctx, tx, pl.ID)
	if err != nil {
		return err
	}
	var pr projects.Project
	var baseURL, model, instructions string
	var sealed []byte
	var review bool
	tasks, name := *pl.Pending == "generate" || *pl.Pending == "revise", "interviewer"
	if tasks {
		name = "task agent"
	}
	err = tx.QueryRow(ctx, `
		select pr.id, pr.github_repo_id, pr.owner, pr.name, pv.base_url, pv.key_enc, ps.review,
			case when $2 then ps.task_model else ps.model end,
			case when $2 then ps.task_instructions else ps.interviewer_instructions end
		from feature_plans fp join projects pr on pr.id = fp.project_id
		join planning_settings ps on ps.user_id = fp.created_by
		join providers pv on pv.id = case when $2 then ps.task_provider_id else ps.provider_id end
		where fp.id = $1`, pl.ID, tasks).Scan(&pr.ID, &pr.RepoID, &pr.Owner, &pr.Name, &baseURL, &sealed, &review, &model, &instructions)
	var failure string // why the turn failed, for people to read
	var doc []section
	var question *message
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		failure = "The plan's creator has no " + name + " model. They pick one in Settings, then someone retries."
	case err != nil:
		return err
	default:
		key, openErr := p.Secrets.Open(sealed)
		if openErr != nil {
			failure = "The plan's creator's provider key cannot be read. They enter it again in Settings, then someone retries."
			break
		}
		// The creator's provider reads the plan, so they must still be able to see it, and generating creates tasks
		// with their token, as board routes do with write access.
		need := projects.Read
		if *pl.Pending == "generate" {
			need = projects.Write
		}
		if failure, err = p.access(ctx, &pr, pl.createdBy, need); err != nil {
			return err
		}
		if failure != "" {
			break
		}
		c := &llm.Client{BaseURL: baseURL, Key: string(key), HTTP: p.LLM}
		if tasks {
			return p.tasks(ctx, tx, c, model, instructions, pr, pl, review)
		}
		tctx, cancel := context.WithTimeout(ctx, turnTimeout)
		doc, question, err = interview(tctx, c, model, instructions, pr.Owner+"/"+pr.Name, pl, msgs)
		cancel()
		if err != nil {
			failure = "The agent's turn failed: " + strings.TrimPrefix(err.Error(), "llm: ") + "."
		}
	}
	if ctx.Err() != nil { // shutting down: the job stays for the next start
		return ctx.Err()
	}

	status, pending := "briefing", (*string)(nil)
	switch {
	case failure != "":
		return fail(ctx, tx, pl.ID, failure)
	case question != nil:
		_, err = tx.Exec(ctx, `insert into plan_messages (plan_id, role, text, options) values ($1, 'agent', $2, $3)`,
			pl.ID, question.Text, question.Options)
		if err != nil {
			return err
		}
	case review:
		status = "needs_review"
	default:
		status, pending = "approved", new("generate")
	}
	_, err = tx.Exec(ctx, `update feature_plans set doc = $2, status = $3, pending = $4, error = null, feedback = null,
		version = version + 1, updated_at = now() where id = $1`, pl.ID, doc, status, pending)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// fail records why plan id's turn failed, in tx, which holds its row, drops its job, for a person to retry, and
// commits.
func fail(ctx context.Context, tx pgx.Tx, id int64, failure string) error {
	_, err := tx.Exec(ctx, `update feature_plans set pending = null, error = $2, version = version + 1, updated_at = now()
		where id = $1`, id, failure)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// access returns why the job cannot run as the plan's creator once their GitHub account no longer has need, Read
// or Write, on the project's repository, checked as project routes check people, and else "". Other errors are
// temporary.
func (p *Planning) access(ctx context.Context, pr *projects.Project, creator int64, need string) (failure string, err error) {
	token, err := p.Credentials.Token(ctx, creator)
	if errors.Is(err, github.ErrNotLinked) {
		return lost(need), nil
	}
	if err != nil {
		return "", err
	}
	perm, err := p.Projects.Permission(ctx, pr, creator, token)
	if unauthorized(err) {
		// GitHub retires a token as it refreshes it, so another caller's refresh since the read may be what it refused.
		current, tErr := p.Credentials.Token(ctx, creator)
		switch {
		case errors.Is(tErr, github.ErrNotLinked):
			return lost(need), nil
		case tErr != nil:
			return "", tErr
		case current == token:
			return lost(need), nil
		}
		if perm, err = p.Projects.Permission(ctx, pr, creator, current); unauthorized(err) {
			return lost(need), nil
		}
	}
	if err != nil {
		return "", err
	}
	if !projects.Allows(perm, need) {
		return lost(need), nil
	}
	return "", nil
}

// lost is why a job that needs need, Read or Write, cannot run as the plan's creator without it.
func lost(need string) string {
	if need == projects.Write {
		return "The plan's creator can no longer push to the repository on GitHub, which creating its tasks needs. Once they can, someone retries."
	}
	return "The plan's creator can no longer see the repository on GitHub. Once they can, someone retries."
}

// unauthorized reports whether err is GitHub refusing the credential itself with 401.
func unauthorized(err error) bool {
	var ghErr *github.Error
	return errors.As(err, &ghErr) && ghErr.Status == http.StatusUnauthorized
}
