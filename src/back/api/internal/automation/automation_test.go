package automation_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/app"
	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/automation"
	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/llm/llmtest"
	"gantt-stop-me/api/internal/projects"
)

// fixture is octo/hello, with one open issue of ada's, added as a project by ada, who can push to it and has a
// provider.
type fixture struct {
	*apptest.Env
	t        *testing.T
	ada      *http.Cookie
	adaID    int64
	token    string // ada's GitHub token
	repo     *githubtest.Repo
	project  int64
	provider int64         // ada's, with the models "luna" and "chat" chosen
	secret   string        // the project's webhook secret, once automated generated it
	llm      *llmtest.Fake // ada's decision and chat models, once withLLM made them the automation's
}

func setup(t *testing.T, configure ...func(*app.Config)) *fixture {
	t.Helper()
	env := apptest.New(t, configure...)
	f := &fixture{Env: env, t: t, repo: env.GitHub.AddRepo("octo", "hello")}
	f.repo.EditIssue(f.repo.AddIssue("One"), func(is *githubtest.Issue) { is.Author = "ada" })
	f.ada, f.adaID, f.token = f.member("ada", "push")
	f.project = env.AddProject(f.ada, "octo", "hello")
	f.provider = f.addProvider(f.adaID, "luna", "chat")
	return f
}

// member signs up login@example.com, links the GitHub account login and grants it role on the repository.
func (f *fixture) member(login, role string) (*http.Cookie, int64, string) {
	f.t.Helper()
	session := f.SignUp(login+"@example.com", "correct horse")
	token := f.LinkGitHub(session, login)
	f.repo.Grant(token, role)
	var id int64
	if err := f.DB.QueryRow(context.Background(), `select id from users where email = $1`, login+"@example.com").Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	return session, id, token
}

// addProvider stores a provider of the user's with these models chosen and returns its id.
func (f *fixture) addProvider(userID int64, models ...string) int64 {
	f.t.Helper()
	var id int64
	err := f.DB.QueryRow(context.Background(), `
		insert into providers (user_id, name, base_url, key_enc, models) values ($1, $2, 'https://llm.test/v1', $3, $4) returning id`,
		userID, fmt.Sprintf("p%d-%d", userID, len(models)), f.Config.Secrets.Seal([]byte("sk-test")), models).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

// path is the URL path of the project's route suffix, such as "/automation".
func (f *fixture) path(suffix string) string {
	return fmt.Sprintf("/api/projects/%d%s", f.project, suffix)
}

// enable turns automation on, or takes it over, as session with provider's "luna" and "chat".
func (f *fixture) enable(session *http.Cookie, provider int64) *apptest.Response {
	f.t.Helper()
	return f.Do("PUT", f.path("/automation"), map[string]any{
		"decisionProviderId": provider, "decisionModel": "luna", "chatProviderId": provider, "chatModel": "chat",
	}, session)
}

// exec runs SQL to set up a test, failing it on error.
func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.DB.Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatal(err)
	}
}

// pausedReason is the project's stored paused_reason, nil while running.
func (f *fixture) pausedReason() *string {
	f.t.Helper()
	var reason *string
	if err := f.DB.QueryRow(context.Background(), `select paused_reason from project_automation where project_id = $1`, f.project).Scan(&reason); err != nil {
		f.t.Fatal(err)
	}
	return reason
}

// automation is the API's Automation over the fixture's database and fake GitHub, for calls no endpoint makes.
func (f *fixture) automation() *automation.Automation {
	return &automation.Automation{
		DB:        f.DB,
		Projects:  &projects.Projects{DB: f.DB, Secrets: f.Config.Secrets, GitHub: f.Config.GitHub},
		Secrets:   f.Config.Secrets,
		PublicURL: f.Config.PublicURL,
		RunnerURL: f.Config.RunnerURL,
	}
}

// settings is GET automation's answer while automation is on.
type settings struct {
	Enabled bool
	Owner   *struct {
		ID    int64
		Login *string
	}
	DecisionProviderID int64
	DecisionModel      string
	ChatProviderID     int64
	ChatModel          string
	PausedReason       *string
	PayloadURL         string
	WebhookSecretSet   bool
}

// show loads the automation settings as session.
func (f *fixture) show(session *http.Cookie) settings {
	f.t.Helper()
	res := f.Do("GET", f.path("/automation"), nil, session)
	res.Expect(http.StatusOK, "")
	var s settings
	res.JSON(&s)
	return s
}

func TestAutomationNeedsRunnerURL(t *testing.T) {
	f := setup(t, func(c *app.Config) { c.RunnerURL = "" })
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		f.Do(method, f.path("/automation"), map[string]any{}, f.ada).Expect(http.StatusNotFound, "not_found")
	}
	f.Do("POST", f.path("/automation/webhook"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("POST", fmt.Sprintf("/api/github/webhook/%d", f.project), "{}").Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path("/tasks/1/runs"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("POST", f.path("/runs/1/cancel"), nil, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Do("POST", f.path("/tasks/1/approve"), map[string]any{}, f.ada).Expect(http.StatusNotFound, "not_found")
	f.Automation.Run(context.Background()) // returns at once instead of polling
}
