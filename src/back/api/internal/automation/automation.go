// Package automation tests a task's linked PR and merges it once a person approves: a project's automation
// settings and owner, the triggers, the queue of test runs, their outcomes, and approval.
package automation

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/board"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/runner"
	"gantt-stop-me/api/internal/secret"
)

// Automation serves the automation routes under /api/projects/{id} and the webhook, polls automated projects'
// pull requests, checks the automation owner before work starts, and runs the queue of test runs.
type Automation struct {
	DB          *pgxpool.Pool
	Accounts    *accounts.Accounts  // guards every route
	Projects    *projects.Projects  // checks access to the project first
	Secrets     *secret.Box         // opens owners' provider keys, and seals and opens webhook secrets
	Credentials *github.Credentials // reads owners' GitHub tokens
	GitHub      *github.Client
	Board       *board.Board   // moves tasks the way the move endpoint does
	PublicURL   string         // base URL users open, without a trailing slash
	RunnerURL   string         // the runner's base URL; automation is off while it is empty
	Runner      *runner.Client // the runner at RunnerURL, which builds and runs workflow executions
	// MaxConcurrentWorkflows is how many workflow executions run at once across the install.
	MaxConcurrentWorkflows int
	LLM                    *http.Client // calls the owners' providers, from llm.HTTPClient

	deliveries sync.WaitGroup // webhook deliveries answered and still being applied
}

// Register adds the automation routes to mux, or none while RunnerURL is empty, so they answer 404 like any unknown route.
func (a *Automation) Register(mux *http.ServeMux) {
	if a.RunnerURL == "" {
		return
	}
	mux.HandleFunc("GET /api/projects/{id}/automation", a.Accounts.Require(a.show))
	mux.HandleFunc("PUT /api/projects/{id}/automation", a.Accounts.Require(a.enable))
	mux.HandleFunc("DELETE /api/projects/{id}/automation", a.Accounts.Require(a.disable))
	mux.HandleFunc("GET /api/projects/{id}/workflows", a.Accounts.Require(a.showWorkflows))
	mux.HandleFunc("PUT /api/projects/{id}/workflows", a.Accounts.Require(a.saveWorkflows))
	mux.HandleFunc("POST /api/projects/{id}/automation/webhook", a.Accounts.Require(a.webhookSecret))
	mux.HandleFunc("POST /api/github/webhook/{projectID}", a.webhook)
	mux.HandleFunc("GET /api/projects/{id}/tasks/{taskID}/runs", a.Accounts.Require(a.listRuns))
	mux.HandleFunc("GET /api/projects/{id}/runs/{runID}", a.Accounts.Require(a.getRun))
	mux.HandleFunc("GET /api/projects/{id}/runs/{runID}/results/{resultID}/screenshot", a.Accounts.Require(a.screenshot))
	mux.HandleFunc("GET /api/projects/{id}/runs/{runID}/results/{resultID}/video", a.Accounts.Require(a.video))
	mux.HandleFunc("POST /api/projects/{id}/runs/{runID}/cancel", a.Accounts.Require(a.cancelRun))
	mux.HandleFunc("POST /api/projects/{id}/tasks/{taskID}/approve", a.Accounts.Require(a.approve))
	mux.HandleFunc("POST /api/projects/{id}/tasks/{taskID}/retest", a.Accounts.Require(a.retest))
	mux.HandleFunc("POST /api/projects/{id}/tasks/{taskID}/request-changes", a.Accounts.Require(a.requestChanges))
	mux.HandleFunc("POST /api/projects/{id}/tasks/{taskID}/takeover", a.Accounts.Require(a.takeOver))
	mux.HandleFunc("DELETE /api/projects/{id}/tasks/{taskID}/takeover", a.Accounts.Require(a.giveBack))
}

// pathID is the path value name as an id, or 0, which matches no row, when it is not a number.
func pathID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id
}

// run is a test run as the run endpoints show it.
type run struct {
	ID         int64      `json:"id"`
	PRNumber   *int       `json:"prNumber"` // nil when no linked pull request was found
	HeadSHA    *string    `json:"headSha"`
	Status     string     `json:"status"`
	Reason     *string    `json:"reason"` // a runner error or cancel reason for the whole run
	CreatedAt  time.Time  `json:"createdAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	ApprovedBy *string    `json:"approvedBy"` // the approver's GitHub login; nil until approved, or once they unlink GitHub
	ApprovedAt *time.Time `json:"approvedAt"` // when a person approved it and its pull request merged
}

// runColumns selects, from test_runs r, a run's fields in order.
const runColumns = `r.id, r.pr_number, r.head_sha, r.status, r.reason, r.created_at, r.finished_at,
	(select login from github_links where user_id = r.approved_by), r.approved_at`

// runNotFound answers 404 for a test run that is not the project's.
func runNotFound(w http.ResponseWriter) {
	httpx.Error(w, http.StatusNotFound, "not_found", "Test run not found.")
}

// Wait waits until the webhook deliveries already answered are applied.
func (a *Automation) Wait() { a.deliveries.Wait() }

// settings is how the API shows a project's automation while it is on.
type settings struct {
	Enabled              bool    `json:"enabled"` // always true; off answers {"enabled": false} alone
	Owner                owner   `json:"owner"`
	DecisionProviderID   int64   `json:"decisionProviderId"`
	DecisionModel        string  `json:"decisionModel"`
	ChatProviderID       int64   `json:"chatProviderId"`
	ChatModel            string  `json:"chatModel"`
	DecisionInstructions string  `json:"decisionInstructions"`
	ChatInstructions     string  `json:"chatInstructions"`
	PausedReason         *string `json:"pausedReason"` // nil while running
	PayloadURL           string  `json:"payloadUrl"`
	WebhookSecretSet     bool    `json:"webhookSecretSet"`
}

// owner is the automation owner as every member may see them: by GitHub login, never by email.
type owner struct {
	ID    int64   `json:"id"`
	Login *string `json:"login"` // nil once they unlink GitHub
}

// settings loads the project's automation settings through db, the pool or a transaction, or pgx.ErrNoRows while it is off.
func (a *Automation) settings(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID int64) (settings, error) {
	s := settings{Enabled: true, PayloadURL: a.payloadURL(projectID)}
	err := db.QueryRow(ctx, `
		select pa.owner_id, gl.login, pa.decision_provider, pa.decision_model, pa.chat_provider, pa.chat_model,
			pa.decision_instructions, pa.chat_instructions, pa.paused_reason, pa.webhook_secret_enc is not null
		from project_automation pa left join github_links gl on gl.user_id = pa.owner_id
		where pa.project_id = $1`, projectID).
		Scan(&s.Owner.ID, &s.Owner.Login, &s.DecisionProviderID, &s.DecisionModel, &s.ChatProviderID, &s.ChatModel,
			&s.DecisionInstructions, &s.ChatInstructions, &s.PausedReason, &s.WebhookSecretSet)
	return s, err
}

// payloadURL is where the repository's webhook sends deliveries for the project.
func (a *Automation) payloadURL(projectID int64) string {
	return fmt.Sprintf("%s/api/github/webhook/%d", a.PublicURL, projectID)
}
