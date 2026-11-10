// Package planning turns a feature someone describes into a plan doc, and the doc into tasks with tests: their
// planning settings, the project's feature plans and their proposed tasks, and the interviewer and task agents the
// planner worker runs between people's answers and reviews.
package planning

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/board"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/secret"
)

// Planning serves /api/me/planning and the plan routes under /api/projects/{id}, and runs the planner worker.
type Planning struct {
	DB          *pgxpool.Pool
	Accounts    *accounts.Accounts  // guards every route
	Projects    *projects.Projects  // checks access to the project first
	Secrets     *secret.Box         // opens the plan creators' provider keys
	Credentials *github.Credentials // reads the plan creators' GitHub tokens
	LLM         *http.Client        // calls the plan creators' providers, from llm.HTTPClient
	Board       *board.Board        // creates the tasks of approved proposals

	mu      sync.Mutex
	running map[int64]bool // the creators whose plans have a turn running
}

// Register adds the planning routes to mux.
func (p *Planning) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/me/planning", p.Accounts.Require(p.showSettings))
	mux.HandleFunc("PUT /api/me/planning", p.Accounts.Require(p.saveSettings))
	mux.HandleFunc("POST /api/projects/{id}/plans", p.Accounts.Require(p.create))
	mux.HandleFunc("GET /api/projects/{id}/plans", p.Accounts.Require(p.list))
	mux.HandleFunc("GET /api/projects/{id}/plans/{planID}", p.Accounts.Require(p.get))
	mux.HandleFunc("PATCH /api/projects/{id}/plans/{planID}", p.Accounts.Require(p.edit))
	mux.HandleFunc("DELETE /api/projects/{id}/plans/{planID}", p.Accounts.Require(p.discard))
	mux.HandleFunc("POST /api/projects/{id}/plans/{planID}/answer", p.Accounts.Require(p.answer))
	mux.HandleFunc("POST /api/projects/{id}/plans/{planID}/draft", p.Accounts.Require(p.draft))
	mux.HandleFunc("POST /api/projects/{id}/plans/{planID}/approve", p.Accounts.Require(p.approve))
	mux.HandleFunc("POST /api/projects/{id}/plans/{planID}/deny", p.Accounts.Require(p.deny))
	mux.HandleFunc("POST /api/projects/{id}/plans/{planID}/retry", p.Accounts.Require(p.retry))
	mux.HandleFunc("PATCH /api/projects/{id}/plans/{planID}/proposals/{proposalID}", p.Accounts.Require(p.editProposal))
	mux.HandleFunc("POST /api/projects/{id}/plans/{planID}/proposals/{proposalID}/approve", p.Accounts.Require(p.approveProposal))
	mux.HandleFunc("POST /api/projects/{id}/plans/{planID}/proposals/{proposalID}/deny", p.Accounts.Require(p.denyProposal))
}

// maxText is the most characters a description, answer, feedback, question or section body can have.
const maxText = 10000

// section is one topic of a plan doc.
type section struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Body  any    `json:"body"`  // text, or a list of items
	State string `json:"state"` // "covered", "asking" or "not_yet"
}

// topics are a doc's sections, in order, as a new plan starts them.
var topics = []section{
	{"goal", "Goal", "", "not_yet"},
	{"users", "Users and use cases", "", "not_yet"},
	{"scope", "Scope", "", "not_yet"},
	{"out_of_scope", "Out of scope", "", "not_yet"},
	{"requirements", "Requirements", "", "not_yet"},
	{"success", "Success criteria", "", "not_yet"},
	{"risks", "Risks", "", "not_yet"},
	{"open_questions", "Open questions", "", "not_yet"},
}

var states = []string{"covered", "asking", "not_yet"}

// change is a new body, a new state, or both, for the doc's section with Key: from a person's edit or update_doc.
type change struct {
	Key   string `json:"key"`
	Body  any    `json:"body"`  // nil keeps the body
	State string `json:"state"` // "" keeps the state
}

// apply returns doc with changes made in order, or an error saying what is wrong with the first it refuses.
func apply(doc []section, changes []change) ([]section, error) {
	doc = slices.Clone(doc)
	for _, c := range changes {
		i := slices.IndexFunc(doc, func(s section) bool { return s.Key == c.Key })
		switch {
		case i < 0:
			return nil, fmt.Errorf("there is no section %q", c.Key)
		case c.Body != nil && !validBody(c.Body):
			return nil, fmt.Errorf("section %q needs a body of text, or a list of texts, of at most %d characters in all", c.Key, maxText)
		case c.State != "" && !slices.Contains(states, c.State):
			return nil, fmt.Errorf("section %q has state %q, not covered, asking or not_yet", c.Key, c.State)
		}
		if c.Body != nil {
			doc[i].Body = c.Body
		}
		if c.State != "" {
			doc[i].State = c.State
		}
	}
	return doc, nil
}

// validBody reports whether b, as JSON decoded it, is text or a list of texts, with no NUL and at most maxText
// characters in all.
func validBody(b any) bool {
	texts, ok := b.([]any)
	if !ok {
		texts = []any{b}
	}
	n := 0
	for _, t := range texts {
		s, ok := t.(string)
		if !ok || strings.ContainsRune(s, 0) {
			return false
		}
		n += utf8.RuneCountInString(s)
	}
	return n <= maxText
}

// text trims s and reports whether it has 1 to max characters and no NUL, which Postgres cannot store.
func text(s string, max int) (string, bool) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	return s, n >= 1 && n <= max && !strings.ContainsRune(s, 0)
}

// plan is a feature plan as the API shows it, without its messages.
type plan struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"projectId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Doc         []section `json:"doc"`
	Pending     *string   `json:"pending"`  // the agent's job, nil while the plan waits on a person
	Feedback    *string   `json:"feedback"` // a denial's, until the agent's next turn takes it
	Error       *string   `json:"error"`    // why the agent's last turn failed, nil since one succeeded
	Version     int       `json:"version"`
	// Answered counts the questions answered or skipped, and ExpectedQuestions is roughly how many the interview takes.
	Answered          int       `json:"answered"`
	ExpectedQuestions int       `json:"expectedQuestions"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`

	asked     int   // the questions the agent asked
	createdBy int64 // the user who started it, 0 once they are deleted
}

// planColumns selects, from feature_plans fp, what scanPlan reads.
const planColumns = `fp.id, fp.project_id, fp.title, fp.description, fp.status, fp.doc, fp.pending, fp.feedback,
	fp.error, fp.version, fp.created_at, fp.updated_at,
	(select count(*) from plan_messages where plan_id = fp.id and role = 'user'),
	(select count(*) from plan_messages where plan_id = fp.id and role = 'agent'), coalesce(fp.created_by, 0)`

// scanPlan reads a plan from a row of planColumns.
func scanPlan(row pgx.Row) (plan, error) {
	var pl plan
	err := row.Scan(&pl.ID, &pl.ProjectID, &pl.Title, &pl.Description, &pl.Status, &pl.Doc, &pl.Pending, &pl.Feedback,
		&pl.Error, &pl.Version, &pl.CreatedAt, &pl.UpdatedAt, &pl.Answered, &pl.asked, &pl.createdBy)
	pl.ExpectedQuestions = pl.asked
	if pl.Status == "briefing" {
		// One more question for each topic the interview has not touched; open questions only collects the rest.
		for _, s := range pl.Doc {
			if s.State == "not_yet" && s.Key != "open_questions" {
				pl.ExpectedQuestions++
			}
		}
		pl.ExpectedQuestions = min(pl.ExpectedQuestions, maxQuestions)
	}
	return pl, err
}

// message is one turn of a plan's interview.
type message struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"` // "agent" or "user"
	Text      string    `json:"text"`
	Options   []string  `json:"options"` // an agent question's suggested answers
	Skipped   bool      `json:"skipped"` // a user message that skips the question instead of answering it
	CreatedAt time.Time `json:"createdAt"`
}

// messages returns the plan's messages in order.
func messages(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, planID int64) ([]message, error) {
	rows, _ := db.Query(ctx, `select id, role, text, options, skipped, created_at from plan_messages where plan_id = $1 order by id`, planID)
	return pgx.CollectRows(rows, pgx.RowToStructByPos[message])
}

// planID is the {planID} path value, or 0, which matches no plan, when it is not a number.
func planID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("planID"), 10, 64)
	return id
}

// show answers the project's plan id with its messages.
func (p *Planning) show(w http.ResponseWriter, r *http.Request, status int, projectID, id int64) {
	out, err := p.load(r.Context(), projectID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "Plan not found.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, status, out)
}

// withMessages is a plan as the plan endpoints answer it.
type withMessages struct {
	plan
	Messages  []message  `json:"messages"`
	Proposals []proposal `json:"proposals"`
}

// load returns the project's plan id with its messages and proposals, or pgx.ErrNoRows.
func (p *Planning) load(ctx context.Context, projectID, id int64) (withMessages, error) {
	pl, err := scanPlan(p.DB.QueryRow(ctx, `select `+planColumns+` from feature_plans fp where fp.id = $1 and fp.project_id = $2`, id, projectID))
	if err != nil {
		return withMessages{}, err
	}
	msgs, err := messages(ctx, p.DB, id)
	if err != nil {
		return withMessages{}, err
	}
	props, err := proposals(ctx, p.DB, id)
	return withMessages{pl, msgs, props}, err
}

// refusal is a change the plan's state refuses, answered with its status and error.
type refusal struct {
	status        int
	code, message string
}

func (r *refusal) Error() string { return r.message }

var (
	errNotFound = &refusal{http.StatusNotFound, "not_found", "Plan not found."}
	errBusy     = &refusal{http.StatusConflict, "agent_busy", "The agent is working on this plan. Load it again once it is done."}
	errConflict = &refusal{http.StatusConflict, "conflict", "This plan changed since you loaded it. It is shown as it is now."}
)

// waiting refuses a change unless the plan is in one of statuses and waits on a person.
func waiting(pl plan, statuses ...string) error {
	if !slices.Contains(statuses, pl.Status) {
		return &refusal{http.StatusConflict, "wrong_status", fmt.Sprintf("This cannot be done to a plan that is %s.", strings.ReplaceAll(pl.Status, "_", " "))}
	}
	if pl.Pending != nil {
		return errBusy
	}
	return nil
}

// change runs edit on the project's plan in the {planID} path value, in a transaction holding the plan's row, then
// bumps its version. The agent's turn holds that row, so a plan it is working on answers 409 agent_busy at once
// instead of waiting for the turn. It answers a *refusal from edit, a conflict with the plan as it is, a
// board.GitHubError as GitHub's, for a call made with token, the caller's credential from Check, or any other error,
// and returns false; on true the caller answers.
func (p *Planning) change(w http.ResponseWriter, r *http.Request, projectID int64, token string,
	edit func(context.Context, pgx.Tx, plan) error,
) bool {
	ctx, id := r.Context(), planID(r)
	err := pgx.BeginFunc(ctx, p.DB, func(tx pgx.Tx) error {
		pl, err := scanPlan(tx.QueryRow(ctx, `select `+planColumns+` from feature_plans fp
			where fp.id = $1 and fp.project_id = $2 for update of fp skip locked`, id, projectID))
		if errors.Is(err, pgx.ErrNoRows) {
			var held bool
			if err := tx.QueryRow(ctx, `select exists (select from feature_plans where id = $1 and project_id = $2)`, id, projectID).Scan(&held); err != nil {
				return err
			}
			if held {
				return errBusy
			}
			return errNotFound
		}
		if err != nil {
			return err
		}
		if err := edit(ctx, tx, pl); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update feature_plans set version = version + 1, updated_at = now() where id = $1`, id)
		return err
	})
	var ref *refusal
	switch {
	case errors.Is(err, errConflict):
		out, err := p.load(ctx, projectID, id)
		if err != nil {
			httpx.Internal(w, r, err)
			return false
		}
		httpx.JSON(w, http.StatusConflict, map[string]any{"error": httpx.ErrorBody{Code: errConflict.code, Message: errConflict.message}, "plan": out})
	case errors.As(err, &ref):
		httpx.Error(w, ref.status, ref.code, ref.message)
	case errors.As(err, new(board.GitHubError)):
		p.Credentials.Fail(w, r, accounts.UserID(ctx), token, err)
	case err != nil:
		httpx.Internal(w, r, err)
	}
	return err == nil
}
