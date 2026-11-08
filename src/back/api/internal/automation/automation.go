// Package automation tests a task's linked PR and merges it once a person approves: a project's automation
// settings and owner, the triggers, the queue of test runs, their outcomes, and approval.
package automation

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/secret"
)

// Automation serves the automation routes under /api/projects/{id} and checks the automation owner before work starts.
type Automation struct {
	DB        *pgxpool.Pool
	Accounts  *accounts.Accounts // guards every route
	Projects  *projects.Projects // checks access to the project first
	Secrets   *secret.Box        // opens owners' GitHub credentials and seals webhook secrets
	GitHub    *github.Client
	PublicURL string // base URL users open, without a trailing slash
	RunnerURL string // the runner's base URL; automation is off while it is empty
}

// Register adds the automation routes to mux, or none while RunnerURL is empty, so they answer 404 like any unknown route.
func (a *Automation) Register(mux *http.ServeMux) {
	if a.RunnerURL == "" {
		return
	}
	mux.HandleFunc("GET /api/projects/{id}/automation", a.Accounts.Require(a.show))
	mux.HandleFunc("PUT /api/projects/{id}/automation", a.Accounts.Require(a.enable))
	mux.HandleFunc("DELETE /api/projects/{id}/automation", a.Accounts.Require(a.disable))
	mux.HandleFunc("POST /api/projects/{id}/automation/webhook", a.Accounts.Require(a.webhookSecret))
}

// settings is how the API shows a project's automation while it is on.
type settings struct {
	Enabled            bool    `json:"enabled"` // always true; off answers {"enabled": false} alone
	Owner              owner   `json:"owner"`
	DecisionProviderID int64   `json:"decisionProviderId"`
	DecisionModel      string  `json:"decisionModel"`
	ChatProviderID     int64   `json:"chatProviderId"`
	ChatModel          string  `json:"chatModel"`
	PausedReason       *string `json:"pausedReason"` // nil while running
	PayloadURL         string  `json:"payloadUrl"`
	WebhookSecretSet   bool    `json:"webhookSecretSet"`
}

// owner is the automation owner as every member may see them: by GitHub login, never by email.
type owner struct {
	ID    int64   `json:"id"`
	Login *string `json:"login"` // nil once they unlink GitHub
}

// settings loads the project's automation settings, or pgx.ErrNoRows while it is off.
func (a *Automation) settings(ctx context.Context, projectID int64) (settings, error) {
	s := settings{Enabled: true, PayloadURL: a.payloadURL(projectID)}
	err := a.DB.QueryRow(ctx, `
		select pa.owner_id, gl.login, pa.decision_provider, pa.decision_model, pa.chat_provider, pa.chat_model,
			pa.paused_reason, pa.webhook_secret_enc is not null
		from project_automation pa left join github_links gl on gl.user_id = pa.owner_id
		where pa.project_id = $1`, projectID).
		Scan(&s.Owner.ID, &s.Owner.Login, &s.DecisionProviderID, &s.DecisionModel, &s.ChatProviderID, &s.ChatModel,
			&s.PausedReason, &s.WebhookSecretSet)
	return s, err
}

// payloadURL is where the repository's webhook sends deliveries for the project.
func (a *Automation) payloadURL(projectID int64) string {
	return fmt.Sprintf("%s/api/github/webhook/%d", a.PublicURL, projectID)
}
