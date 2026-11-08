package planning_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/llm/llmtest"
	"gantt-stop-me/api/internal/tasktests"
)

// fixture is octo/hello added as a project by ada, who can push to it and plans with the fake provider's chat
// model, review on.
type fixture struct {
	*apptest.Env
	t        *testing.T
	ada      *http.Cookie
	token    string // ada's GitHub token
	repo     *githubtest.Repo
	project  int64
	llm      *llmtest.Fake
	provider int64 // ada's, the fake with all its models chosen
}

func setup(t *testing.T) *fixture {
	t.Helper()
	env := apptest.New(t)
	f := &fixture{Env: env, t: t, repo: env.GitHub.AddRepo("octo", "hello"), llm: llmtest.New(t)}
	f.ada = env.SignUp("ada@example.com", "correct horse")
	f.token = env.LinkGitHub(f.ada, "ada")
	f.repo.Grant(f.token, "push")
	f.project = env.AddProject(f.ada, "octo", "hello")
	f.provider = f.addProvider(f.ada)
	f.Do("PUT", "/api/me/planning", agents(f.provider, llmtest.ChatModel, true), f.ada).Expect(http.StatusOK, "")
	return f
}

// agents is a PUT /api/me/planning body giving both agents provider and model, nil for none.
func agents(provider, model any, review bool) map[string]any {
	a := map[string]any{"providerId": provider, "model": model}
	return map[string]any{"review": review, "interviewer": a, "taskAgent": a}
}

// member signs up login@example.com, links the GitHub account login and grants it role on the repository.
func (f *fixture) member(login, role string) *http.Cookie {
	f.t.Helper()
	session := f.SignUp(login+"@example.com", "correct horse")
	f.repo.Grant(f.LinkGitHub(session, login), role)
	return session
}

// addProvider adds the fake provider as one of session's, with all its models chosen, and returns its id.
func (f *fixture) addProvider(session *http.Cookie) int64 {
	f.t.Helper()
	res := f.Do("POST", "/api/providers", map[string]any{"name": "Fake", "baseUrl": f.llm.URL, "apiKey": f.llm.Key, "models": f.llm.Models}, session)
	res.Expect(http.StatusCreated, "")
	var p struct{ ID int64 }
	res.JSON(&p)
	return p.ID
}

// path is the URL path of the project's route suffix, such as "/plans/%d".
func (f *fixture) path(suffix string, args ...any) string {
	return fmt.Sprintf("/api/projects/%d", f.project) + fmt.Sprintf(suffix, args...)
}

type planJSON struct {
	ID, ProjectID                        int64
	Title, Description, Status           string
	Doc                                  []sectionJSON
	Pending, Feedback, Error             *string
	Version, Answered, ExpectedQuestions int
	Messages                             []messageJSON
	Proposals                            []proposalJSON
}

type proposalJSON struct {
	ID                 int64
	Position           int
	Status             string
	Title, Description string
	AcceptanceCriteria []string
	Tests              []tasktests.Test
	DependsOn          []int
	Source, AgentNote  string
	Feedback           *string
	TaskID             *int64
}

// sectionJSON has the API's field names, so a test can send a doc back as GET answered it.
type sectionJSON struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Body  any    `json:"body"` // a string, or []any of strings
	State string `json:"state"`
}

type messageJSON struct {
	ID         int64
	Role, Text string
	Options    []string
	Skipped    bool
}

// section returns the plan's section with key.
func (pl planJSON) section(key string) sectionJSON {
	i := slices.IndexFunc(pl.Doc, func(s sectionJSON) bool { return s.Key == key })
	if i < 0 {
		return sectionJSON{}
	}
	return pl.Doc[i]
}

// decode reads the plan res answers, failing the test unless it has status.
func decode(t *testing.T, res *apptest.Response, status int) planJSON {
	t.Helper()
	res.Expect(status, "")
	var pl planJSON
	res.JSON(&pl)
	return pl
}

// start starts a plan as ada and returns it.
func (f *fixture) start(title, description string) planJSON {
	f.t.Helper()
	return decode(f.t, f.Do("POST", f.path("/plans"), map[string]string{"title": title, "description": description}, f.ada), http.StatusCreated)
}

// plan loads plan id as ada.
func (f *fixture) plan(id int64) planJSON {
	f.t.Helper()
	return decode(f.t, f.Do("GET", f.path("/plans/%d", id), nil, f.ada), http.StatusOK)
}

// act posts body to plan id's action, such as "answer", as ada.
func (f *fixture) act(id int64, action string, body any) *apptest.Response {
	f.t.Helper()
	return f.Do("POST", f.path("/plans/%d/%s", id, action), body, f.ada)
}

// work runs one pass of the planner worker.
func (f *fixture) work() { f.Planning.Work(context.Background()) }

// prompt is the first message of the nth chat request the fake received, from 0.
func (f *fixture) prompt(n int) string {
	f.t.Helper()
	chats := f.llm.Chats()
	if len(chats) <= n {
		f.t.Fatalf("%d chat requests, want over %d", len(chats), n)
	}
	return chats[n].Messages[0].Content
}

// toReview starts a plan the agent finishes at once, with the goal and scope covered, and returns it in needs_review.
// No other plan may have a job pending, or its turn takes the scripted reply first.
func (f *fixture) toReview() planJSON {
	f.t.Helper()
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.llm.Reply(reply(update(sec("goal", "Export the board.", "covered"), sec("scope", []string{"CSV"}, "covered")), finish))
	f.work()
	if pl = f.plan(pl.ID); pl.Status != "needs_review" {
		f.t.Fatalf("plan %+v, want it in needs_review", pl)
	}
	return pl
}

func reply(calls ...llm.FunctionCall) llmtest.Reply { return llmtest.Reply{Calls: calls} }

func call(name string, args any) llm.FunctionCall {
	data, _ := json.Marshal(args)
	return llm.FunctionCall{Name: name, Arguments: string(data)}
}

func ask(question string, options ...string) llm.FunctionCall {
	return call("ask", map[string]any{"question": question, "options": options})
}

// sec is one section change of update_doc; a nil body or "" state leaves it out.
func sec(key string, body any, state string) map[string]any {
	s := map[string]any{"key": key}
	if body != nil {
		s["body"] = body
	}
	if state != "" {
		s["state"] = state
	}
	return s
}

func update(sections ...map[string]any) llm.FunctionCall {
	return call("update_doc", map[string]any{"sections": sections})
}

var finish = llm.FunctionCall{Name: "finish", Arguments: "{}"}

// orNull is s, or "null" for nil, to compare the API's nullable strings.
func orNull(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

func TestPlansNeedAccess(t *testing.T) {
	f := setup(t)
	pl := f.generate(f.approved(), task("Export", nil, script("make")))
	proposal := f.path("/plans/%d/proposals/%d", pl.ID, pl.Proposals[0].ID)
	reader := f.member("bob", "pull")
	stranger := f.SignUp("eve@example.com", "correct horse")
	f.LinkGitHub(stranger, "eve")

	f.Do("GET", f.path("/plans"), nil, reader).Expect(http.StatusOK, "")
	if got := decode(t, f.Do("GET", f.path("/plans/%d", pl.ID), nil, reader), http.StatusOK); len(got.Proposals) != 1 {
		t.Fatalf("plan %+v as a reader sees it, want its proposal", got)
	}
	writes := []struct{ method, path string }{
		{"PATCH", proposal},
		{"POST", proposal + "/approve"},
		{"POST", proposal + "/deny"},
		{"POST", f.path("/plans")},
		{"PATCH", f.path("/plans/%d", pl.ID)},
		{"DELETE", f.path("/plans/%d", pl.ID)},
		{"POST", f.path("/plans/%d/answer", pl.ID)},
		{"POST", f.path("/plans/%d/draft", pl.ID)},
		{"POST", f.path("/plans/%d/approve", pl.ID)},
		{"POST", f.path("/plans/%d/deny", pl.ID)},
		{"POST", f.path("/plans/%d/retry", pl.ID)},
	}
	for _, w := range writes {
		f.Do(w.method, w.path, map[string]any{}, reader).Expect(http.StatusForbidden, "forbidden")
		f.Do(w.method, w.path, map[string]any{}, stranger).Expect(http.StatusNotFound, "not_found")
		f.Do(w.method, w.path, map[string]any{}).Expect(http.StatusUnauthorized, "unauthorized")
	}
	f.Do("GET", f.path("/plans"), nil, stranger).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path("/plans/%d", pl.ID), nil, stranger).Expect(http.StatusNotFound, "not_found")
	f.Do("GET", f.path("/plans"), nil).Expect(http.StatusUnauthorized, "unauthorized")
	if got := f.plan(pl.ID); got.Status != "approved" || got.Version != pl.Version || got.Proposals[0].Status != "needs_review" {
		t.Fatalf("plan %+v after refused writes, want it unchanged", got)
	}
	if n := f.tasks(); n != 0 {
		t.Fatalf("%d tasks after refused writes", n)
	}
}
