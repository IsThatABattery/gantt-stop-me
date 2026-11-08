package planning

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/tasktests"
)

// maxProposals is the most proposals the task agent makes for one plan.
const maxProposals = 30

// proposal is a task the task agent proposed for an approved plan.
type proposal struct {
	ID                 int64            `json:"id"`
	Position           int              `json:"position"` // from 1, in the order the agent proposed them
	Status             string           `json:"status"`   // "needs_review", "approved", "approved_edited", "sent_back" or "dropped"
	Title              string           `json:"title"`
	Description        string           `json:"description"`
	AcceptanceCriteria []string         `json:"acceptanceCriteria"`
	Tests              []tasktests.Test `json:"tests"`
	DependsOn          []int            `json:"dependsOn"` // the positions of earlier proposals it needs
	Source             string           `json:"source"`    // the doc section it came from
	AgentNote          string           `json:"agentNote"`
	Feedback           *string          `json:"feedback"` // a denial's: a revision takes it, a drop keeps it
	TaskID             *int64           `json:"taskId"`   // the task created on approval
	CreatedAt          time.Time        `json:"createdAt"`
	Edited             bool             `json:"-"` // a person edited it since the agent wrote it
}

// proposalColumns selects, from plan_proposals, proposal's fields in order.
const proposalColumns = `id, position, status, title, description, acceptance_criteria, tests, depends_on, source,
	agent_note, feedback, task_id, created_at, edited`

// proposals returns the plan's proposals by position.
func proposals(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, planID int64) ([]proposal, error) {
	rows, _ := db.Query(ctx, `select `+proposalColumns+` from plan_proposals where plan_id = $1 order by position`, planID)
	return pgx.CollectRows(rows, pgx.RowToStructByPos[proposal])
}

// proposalID is the {proposalID} path value, or 0, which matches no proposal, when it is not a number.
func proposalID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("proposalID"), 10, 64)
	return id
}

// reviewing returns plan pl's proposal in the {proposalID} path value, for a person to act on: it must be in one of
// statuses, and pl must be approved and wait on a person.
func reviewing(ctx context.Context, tx pgx.Tx, r *http.Request, pl plan, statuses ...string) (proposal, error) {
	rows, _ := tx.Query(ctx, `select `+proposalColumns+` from plan_proposals where id = $1 and plan_id = $2`, proposalID(r), pl.ID)
	pp, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[proposal])
	if errors.Is(err, pgx.ErrNoRows) {
		return proposal{}, &refusal{http.StatusNotFound, "not_found", "Proposal not found."}
	}
	if err != nil {
		return proposal{}, err
	}
	if err := waiting(pl, "approved"); err != nil {
		return proposal{}, err
	}
	if !slices.Contains(statuses, pp.Status) {
		return proposal{}, &refusal{http.StatusConflict, "wrong_status",
			fmt.Sprintf("This cannot be done to a proposal that is %s.", strings.ReplaceAll(pp.Status, "_", " "))}
	}
	return pp, nil
}

// invalid refuses a proposal, as a person sent it or the agent proposed it, saying why.
func invalid(why string) error { return &refusal{http.StatusBadRequest, "invalid_proposal", why} }

// tidy trims pp's text and puts its tests as GitHub reads them back, or refuses it with invalid: it needs a title of
// 1 to 256 characters on one line, a description, at least one acceptance criterion of one line each, at least one
// valid test, and dependencies on earlier proposals only.
func tidy(pp *proposal) error {
	var ok bool
	if pp.Title, ok = text(pp.Title, 256); !ok || strings.ContainsAny(pp.Title, "\r\n") {
		return invalid("A title needs 1 to 256 characters, on one line.")
	}
	if pp.Description, ok = text(pp.Description, maxText); !ok {
		return invalid(fmt.Sprintf("A description needs 1 to %d characters.", maxText))
	}
	criteria := []string{} // not nil, which would store null
	for i, c := range pp.AcceptanceCriteria {
		if c, ok = text(c, maxText); !ok || strings.ContainsAny(c, "\r\n") {
			return invalid(fmt.Sprintf("Acceptance criterion %d needs 1 to %d characters, on one line.", i+1, maxText))
		}
		criteria = append(criteria, c)
	}
	if pp.AcceptanceCriteria = criteria; len(criteria) == 0 {
		return invalid("A task needs at least one acceptance criterion.")
	}
	for _, s := range []*string{&pp.Source, &pp.AgentNote} {
		if *s = strings.TrimSpace(*s); utf8.RuneCountInString(*s) > maxText || strings.ContainsRune(*s, 0) {
			return invalid(fmt.Sprintf("A source and a note can have at most %d characters each.", maxText))
		}
	}
	if len(pp.Tests) == 0 {
		return invalid("A task needs at least one test.")
	}
	if err := tasktests.Validate(pp.Tests); err != nil {
		return invalid(err.Error())
	}
	// As GitHub reads them back, so the card holds exactly these. Checked again, because a name that is only a
	// [name] pin reads back as no name.
	pp.Tests, _ = tasktests.Parse(tasktests.Render(pp.Tests))
	if err := tasktests.Validate(pp.Tests); err != nil {
		return invalid(err.Error())
	}
	pp.DependsOn = slices.Compact(slices.Sorted(slices.Values(pp.DependsOn)))
	if pp.DependsOn == nil {
		pp.DependsOn = []int{}
	}
	for _, d := range pp.DependsOn {
		if d < 1 || d >= pp.Position {
			return invalid(fmt.Sprintf("A task can depend only on tasks before it, by number, and %d is not one.", d))
		}
	}
	return nil
}

// body returns the issue body proposal pp of plan planID gets now: its description; its acceptance criteria as a
// checklist; a "Depends on" line for each proposal it needs that is not dropped, naming that one's issue once it
// has its task and its title until then; and its tests' section. It refuses, with invalid, a body whose Tests
// section is not exactly the tests' or that is too long for GitHub.
func body(ctx context.Context, tx pgx.Tx, planID int64, pp proposal) (string, error) {
	rows, _ := tx.Query(ctx, `
		select coalesce('#' || t.issue_number, d.title) from plan_proposals d left join tasks t on t.id = d.task_id
		where d.plan_id = $1 and d.position = any($2) and d.status <> 'dropped' order by d.position`, planID, pp.DependsOn)
	refs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(pp.Description + "\n\n## Acceptance criteria\n")
	for _, c := range pp.AcceptanceCriteria {
		b.WriteString("- [ ] " + c + "\n")
	}
	if len(refs) > 0 {
		b.WriteString("\n")
	}
	for _, ref := range refs {
		b.WriteString("Depends on " + ref + "\n")
	}
	b.WriteString("\n" + tasktests.Render(pp.Tests))
	s := b.String()
	if tasktests.Section(s) != tasktests.Render(pp.Tests) {
		return "", invalid("The description and acceptance criteria must leave the Tests section to the tests: give " +
			"them no Tests heading, and close every code fence.")
	}
	if n := utf8.RuneCountInString(s); n > 65536 {
		return "", invalid(fmt.Sprintf("The task's issue would have %d characters, more than GitHub's 65536.", n))
	}
	return s, nil
}

// write saves proposal pp of plan planID in tx: a new one when it has no id yet.
func write(ctx context.Context, tx pgx.Tx, planID int64, pp proposal) error {
	if pp.ID == 0 {
		_, err := tx.Exec(ctx, `
			insert into plan_proposals (plan_id, position, status, title, description, acceptance_criteria, tests, depends_on, source, agent_note)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			planID, pp.Position, pp.Status, pp.Title, pp.Description, pp.AcceptanceCriteria, pp.Tests, pp.DependsOn, pp.Source, pp.AgentNote)
		return err
	}
	_, err := tx.Exec(ctx, `
		update plan_proposals set status = $2, title = $3, description = $4, acceptance_criteria = $5, tests = $6, depends_on = $7,
			source = $8, agent_note = $9, feedback = $10, edited = $11
		where id = $1`,
		pp.ID, pp.Status, pp.Title, pp.Description, pp.AcceptanceCriteria, pp.Tests, pp.DependsOn, pp.Source, pp.AgentNote, pp.Feedback, pp.Edited)
	return err
}

// createTask creates proposal pp's task in project pr with token, at the end of Backlog, and records it on pp as
// approved, or approved_edited after a person's edit, in tx, which holds the row of pp's plan planID. It takes the
// project's lock, so call it under projects.Serialize, and commit tx only when it succeeds: the task and its record
// commit together, with no second connection. The card takes the issue's tests, which are pp's. ctx must outlive the
// person's request, since a task GitHub has must be recorded.
func (p *Planning) createTask(ctx context.Context, tx pgx.Tx, token string, pr projects.Project, planID int64, pp proposal) error {
	if err := projects.Lock(ctx, tx, pr.ID); err != nil {
		return err
	}
	b, err := body(ctx, tx, planID, pp)
	if err != nil {
		return err
	}
	id, err := p.Board.CreateTask(ctx, tx, token, pr, pp.Title, b)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `update plan_proposals set task_id = $2, status = case when edited then 'approved_edited' else 'approved' end
		where id = $1`, pp.ID, id)
	return err
}

// settle makes plan id done once its tasks are settled: it is approved with no job, has proposals, and each one
// not dropped has its task. A failed turn's error no longer matters then.
func settle(ctx context.Context, tx pgx.Tx, id int64) error {
	_, err := tx.Exec(ctx, `
		update feature_plans fp set status = 'done', error = null
		where id = $1 and status = 'approved' and pending is null and exists (select from plan_proposals where plan_id = fp.id)
			and not exists (select from plan_proposals where plan_id = fp.id and status <> 'dropped' and task_id is null)`, id)
	return err
}
