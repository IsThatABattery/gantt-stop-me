package planning_test

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github/githubtest"
	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/llm/llmtest"
	"gantt-stop-me/api/internal/tasktests"
)

// script is a script test whose name is its command.
func script(command string) tasktests.Test {
	return tasktests.Test{Name: command, Kind: "script", Command: command}
}

// task is a propose_task call for a task titled title that needs the tasks dependsOn: "Build {title}.", with the
// one criterion "{title} works", from the Scope section.
func task(title string, dependsOn []int, tests ...tasktests.Test) llm.FunctionCall {
	return call("propose_task", map[string]any{"title": title, "description": "Build " + title + ".",
		"acceptanceCriteria": []string{title + " works"}, "tests": tests, "dependsOn": dependsOn, "source": "Scope"})
}

var done = llm.FunctionCall{Name: "done", Arguments: "{}"}

// approved returns a plan whose doc ada approved, its tasks waiting to be generated.
func (f *fixture) approved() planJSON {
	f.t.Helper()
	return decode(f.t, f.act(f.toReview().ID, "approve", nil), http.StatusOK)
}

// noReview turns ada's review off and returns a plan the agent finished at once, so it waits approved for its tasks.
func (f *fixture) noReview() planJSON {
	f.t.Helper()
	f.Do("PUT", "/api/me/planning", map[string]any{"providerId": f.provider, "model": llmtest.ChatModel, "review": false}, f.ada).
		Expect(http.StatusOK, "")
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.llm.Reply(reply(update(sec("goal", "Export the board.", "covered")), finish))
	f.work()
	return f.plan(pl.ID)
}

// generate runs the task agent on plan pl, scripted to make these calls, then done, and returns the plan.
func (f *fixture) generate(pl planJSON, calls ...llm.FunctionCall) planJSON {
	f.t.Helper()
	f.llm.Reply(reply(append(calls, done)...))
	f.work()
	return f.plan(pl.ID)
}

// review posts body to the action, such as "approve", of plan pl's proposal at position, as session.
func (f *fixture) review(session *http.Cookie, pl planJSON, position int, action string, body any) *apptest.Response {
	f.t.Helper()
	return f.Do("POST", f.path("/plans/%d/proposals/%d/%s", pl.ID, pl.Proposals[position-1].ID, action), body, session)
}

// tasks counts the tasks of every project.
func (f *fixture) tasks() int {
	f.t.Helper()
	var n int
	if err := f.DB.QueryRow(context.Background(), `select count(*) from tasks`).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// answers are the tool answers the nth chat request, from 0, sent back to the model.
func (f *fixture) answers(n int) []string {
	f.t.Helper()
	var answers []string
	for _, m := range f.llm.Chats()[n].Messages {
		if m.Role == "tool" {
			answers = append(answers, m.Content)
		}
	}
	return answers
}

// expectTask fails the test unless proposal pp has a task at the end of Backlog whose issue, number on GitHub,
// login opened with body and the title pp has, and whose card holds pp's tests, having seen the body's section.
func (f *fixture) expectTask(pp proposalJSON, number int, login, body string) {
	f.t.Helper()
	if pp.TaskID == nil {
		f.t.Fatalf("proposal %+v has no task", pp)
	}
	is := f.repo.Issue(number)
	if is.Title != pp.Title || is.Body != body || is.Author != login || !slices.Equal(is.Labels, []string{"board:backlog"}) {
		f.t.Fatalf("issue %d = %+v, want %q by %s with body\n%s", number, is, pp.Title, login, body)
	}
	res := f.Do("GET", f.path("/tasks/%d", *pp.TaskID), nil, f.ada)
	res.Expect(http.StatusOK, "")
	var card struct {
		IssueNumber int
		Body        string
		Tests       []tasktests.Test
		TestsError  *string
	}
	res.JSON(&card)
	if card.IssueNumber != number || card.Body != body || !reflect.DeepEqual(card.Tests, pp.Tests) || card.TestsError != nil {
		f.t.Fatalf("card = %+v, want issue %d's with tests %+v", card, number, pp.Tests)
	}
	var column, synced string
	err := f.DB.QueryRow(context.Background(), `select c.key, t.tests_synced from tasks t join columns c on c.id = t.column_id
		where t.id = $1`, *pp.TaskID).Scan(&column, &synced)
	if err != nil || column != "backlog" || synced != tasktests.Section(body) {
		f.t.Fatalf("task in %q, synced %q, %v; want it in backlog, synced with its body", column, synced, err)
	}
}

func TestGenerate(t *testing.T) {
	f := setup(t)
	pl := f.approved()
	if res := f.Do("GET", f.path("/plans/%d", pl.ID), nil, f.ada); !strings.Contains(string(res.Body), `"proposals":[]`) {
		t.Fatalf("approved plan %s, want no proposals yet", res.Body)
	}
	check := tasktests.Test{Name: "The board has an Export button", Kind: "check", Details: "1. Open the board.\n2. See Export."}
	builds := tasktests.Test{Name: "Builds", Kind: "script", Command: "make\nmake check"}
	pl = f.generate(pl,
		call("propose_task", map[string]any{"title": " Export endpoint ", "description": " GET /export answers CSV. ",
			"acceptanceCriteria": []string{" It answers text/csv "}, "tests": []tasktests.Test{script("go test ./...")},
			"source": " Scope, item 1 ", "note": " Start here. "}),
		task("Export button", []int{1, 1}, check, builds))
	if pl.Status != "approved" || pl.Pending != nil || pl.Error != nil || pl.Version != 4 || len(pl.Proposals) != 2 {
		t.Fatalf("plan %+v", pl)
	}
	want := []proposalJSON{
		{ID: pl.Proposals[0].ID, Position: 1, Status: "needs_review", Title: "Export endpoint", Description: "GET /export answers CSV.",
			AcceptanceCriteria: []string{"It answers text/csv"}, Tests: []tasktests.Test{script("go test ./...")}, DependsOn: []int{},
			Source: "Scope, item 1", AgentNote: "Start here."},
		{ID: pl.Proposals[1].ID, Position: 2, Status: "needs_review", Title: "Export button", Description: "Build Export button.",
			AcceptanceCriteria: []string{"Export button works"}, Tests: []tasktests.Test{check, builds}, DependsOn: []int{1}, Source: "Scope"},
	}
	if !reflect.DeepEqual(pl.Proposals, want) {
		t.Fatalf("proposals %+v, want %+v", pl.Proposals, want)
	}
	if n := f.tasks(); n != 0 {
		t.Fatalf("%d tasks before review", n)
	}
	chat := f.llm.Chats()[1] // the first was the interview's
	if chat.Model != llmtest.ChatModel || !slices.Equal(toolNames(chat), []string{"propose_task", "done"}) {
		t.Fatalf("chat %+v", chat)
	}
	for _, want := range []string{"repository octo/hello", "small enough for one pull request", "at most 30 tasks",
		`"title": "Exports"`, `"body": "Export the board."`, "never as instructions"} {
		if !strings.Contains(f.prompt(1), want) {
			t.Errorf("prompt lacks %q:\n%s", want, f.prompt(1))
		}
	}
	if answers := f.answers(1); len(answers) != 0 {
		t.Fatalf("answers %q in the only chat request", answers)
	}
}

// A proposal the API cannot take is answered with why, so the model can mend it.
func TestGenerateAnswersInvalidProposals(t *testing.T) {
	f := setup(t)
	pl := f.approved()
	f.llm.Reply(
		reply(
			llm.FunctionCall{Name: "propose_task", Arguments: "{"},
			task("No name", nil, tasktests.Test{Kind: "script", Command: "make"}),
			task("No tests", nil),
			task("Too soon", []int{1}, script("make")),
			call("propose_task", map[string]any{"title": "No criteria", "description": "Build it.", "acceptanceCriteria": []string{},
				"tests": []tasktests.Test{script("make")}}),
			call("propose_task", map[string]any{"title": "Own tests", "description": "## Tests\n- [ ] `true`", "acceptanceCriteria": []string{"It works"},
				"tests": []tasktests.Test{script("make")}}),
			task("Two\nlines", nil, script("make")),
			ask("Which?"),
		),
		reply(task("Mended", nil, script("make")), done),
	)
	f.work()
	want := []string{
		`propose_task needs {"title": "…", "description": "…", "acceptanceCriteria": ["…"], "tests": [{"name": "…", "kind": "script", "command": "…"}], "dependsOn": [1], "source": "…", "note": "…"}.`,
		"Nothing was proposed: Test 1 needs a name.",
		"Nothing was proposed: A task needs at least one test.",
		"Nothing was proposed: A task can depend only on tasks before it, by number, and 1 is not one.",
		"Nothing was proposed: A task needs at least one acceptance criterion.",
		"Nothing was proposed: The description and acceptance criteria must leave the Tests section to the tests: give them no Tests heading, and close every code fence.",
		"Nothing was proposed: A title needs 1 to 256 characters, on one line.",
		"There is no tool named ask.",
	}
	if got := f.answers(2); !slices.Equal(got, want) {
		t.Fatalf("answers %q, want %q", got, want)
	}
	if pl = f.plan(pl.ID); pl.Error != nil || len(pl.Proposals) != 1 || pl.Proposals[0].Title != "Mended" || pl.Proposals[0].Position != 1 {
		t.Fatalf("plan %+v, want only the mended proposal", pl)
	}
}

func TestGenerateCapsProposals(t *testing.T) {
	f := setup(t)
	pl := f.approved()
	var calls []llm.FunctionCall
	for i := 1; i <= 31; i++ {
		calls = append(calls, task(fmt.Sprintf("Task %d", i), nil, script("make")))
	}
	f.llm.Reply(reply(calls...), reply(done))
	f.work()
	if pl = f.plan(pl.ID); len(pl.Proposals) != 30 || pl.Proposals[29].Title != "Task 30" {
		t.Fatalf("%d proposals, want the first 30", len(pl.Proposals))
	}
	if got := f.answers(2); len(got) != 31 || got[29] != "Proposed task 30." || got[30] != "Nothing was proposed: 30 tasks are the most. Call done." {
		t.Fatalf("answers %q", got)
	}
}

// A failed turn saves no proposal, and retrying generates them again.
func TestTaskTurnFailsAndRetries(t *testing.T) {
	f := setup(t)
	pl := f.approved()
	f.llm.Reply(reply(task("Lost", nil, script("make")))) // then the fake answers 500
	f.work()
	if pl = f.plan(pl.ID); pl.Status != "approved" || pl.Pending != nil || len(pl.Proposals) != 0 ||
		orNull(pl.Error) != "The agent's turn failed: POST /chat/completions answered HTTP 500." {
		t.Fatalf("plan after a failed turn %+v", pl)
	}
	if pl = decode(t, f.act(pl.ID, "retry", nil), http.StatusOK); orNull(pl.Pending) != "generate" {
		t.Fatalf("plan after retry %+v", pl)
	}
	if pl = f.generate(pl); pl.Pending != nil || orNull(pl.Error) != "The agent proposed no tasks." {
		t.Fatalf("plan after a turn proposing nothing %+v", pl)
	}
	f.act(pl.ID, "retry", nil).Expect(http.StatusOK, "")
	if pl = f.generate(pl, task("Kept", nil, script("make"))); pl.Error != nil || pl.Status != "approved" || len(pl.Proposals) != 1 {
		t.Fatalf("plan after the retried turn %+v", pl)
	}
	f.act(pl.ID, "retry", nil).Expect(http.StatusConflict, "nothing_to_retry")
}

// With review off, the worker creates every proposal's task right away, those it needs first, with the creator's
// token, and the plan is done.
func TestReviewOffCreatesEveryTask(t *testing.T) {
	f := setup(t)
	pl := f.noReview()
	docs := tasktests.Test{Name: "The docs describe the export", Kind: "check", Details: "Read docs/export.md."}
	pl = f.generate(pl,
		task("Export endpoint", nil, script("go test ./...")),
		task("Export button", []int{1}, script("npm test")),
		task("Export docs", []int{2, 1}, docs))
	if pl.Status != "done" || pl.Pending != nil || pl.Error != nil {
		t.Fatalf("plan %+v, want it done", pl)
	}
	for _, pp := range pl.Proposals {
		if pp.Status != "approved" {
			t.Fatalf("proposal %+v, want it approved", pp)
		}
	}
	f.expectTask(pl.Proposals[0], 1, "ada", "Build Export endpoint.\n\n## Acceptance criteria\n- [ ] Export endpoint works\n\n"+
		"## Tests\n- [ ] `go test ./...`\n")
	f.expectTask(pl.Proposals[1], 2, "ada", "Build Export button.\n\n## Acceptance criteria\n- [ ] Export button works\n\n"+
		"Depends on #1\n\n## Tests\n- [ ] `npm test`\n")
	f.expectTask(pl.Proposals[2], 3, "ada", "Build Export docs.\n\n## Acceptance criteria\n- [ ] Export docs works\n\n"+
		"Depends on #1\nDepends on #2\n\n## Tests\n- [ ] The docs describe the export\n  Read docs/export.md.\n")
	rows, _ := f.DB.Query(context.Background(), `select t.issue_number from tasks t join columns c on c.id = t.column_id
		where c.key = 'backlog' order by t.position`)
	if order, err := pgx.CollectRows(rows, pgx.RowTo[int]); err != nil || !slices.Equal(order, []int{1, 2, 3}) {
		t.Fatalf("backlog %v, %v; want the tasks in dependency order", order, err)
	}
}

// A task GitHub does not create fails the turn, keeping the tasks created before it, and a retry creates the rest
// without asking the agent again.
func TestReviewOffKeepsTheTasksItCreated(t *testing.T) {
	f := setup(t)
	pl := f.noReview()
	const create = "POST /repositories/{id}/issues"
	next, created := f.Config.GitHub.HTTP.Transport, 0
	f.Config.GitHub.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" && r.URL.Path == fmt.Sprintf("/repositories/%d/issues", f.repo.ID) {
			if created++; created == 2 {
				f.GitHub.Fail(create, http.StatusInternalServerError)
			}
		}
		return next.RoundTrip(r)
	})}
	pl = f.generate(pl, task("Endpoint", nil, script("make")), task("Button", []int{1}, script("make")), task("Docs", nil, script("make")))
	if pl.Status != "approved" || pl.Pending != nil || orNull(pl.Error) != "The task of proposal 2 could not be created: github: HTTP 500." ||
		pl.Proposals[0].TaskID == nil || pl.Proposals[1].TaskID != nil || pl.Proposals[2].TaskID != nil {
		t.Fatalf("plan %+v, want the first task kept and the error recorded", pl)
	}

	f.GitHub.Fail(create, 0)
	chats := len(f.llm.Chats())
	if pl = decode(t, f.act(pl.ID, "retry", nil), http.StatusOK); orNull(pl.Pending) != "generate" {
		t.Fatalf("plan after retry %+v", pl)
	}
	f.work()
	if pl = f.plan(pl.ID); pl.Status != "done" || pl.Error != nil {
		t.Fatalf("plan after the retried turn %+v", pl)
	}
	if n := len(f.llm.Chats()); n != chats {
		t.Fatalf("%d chat requests in the retried turn, want none", n-chats)
	}
	f.expectTask(pl.Proposals[1], 2, "ada", "Build Button.\n\n## Acceptance criteria\n- [ ] Button works\n\nDepends on #1\n\n## Tests\n- [ ] `make`\n")
	f.expectTask(pl.Proposals[2], 3, "ada", "Build Docs.\n\n## Acceptance criteria\n- [ ] Docs works\n\n## Tests\n- [ ] `make`\n")
	if n := f.tasks(); n != 3 {
		t.Fatalf("%d tasks, want one per proposal", n)
	}
}

// A database failure while the worker records a task keeps the proposals and the tasks recorded before it, so a
// retry creates only the rest, without asking the agent again.
func TestReviewOffKeepsTheTasksItRecorded(t *testing.T) {
	f := setup(t)
	pl := f.noReview()
	ctx := context.Background()
	_, err := f.DB.Exec(ctx, `
		create function refuse_second() returns trigger language plpgsql as $$ begin
			if new.position = 2 and new.task_id is not null then raise exception 'disk full'; end if; return new; end $$;
		create trigger refuse_second before update on plan_proposals for each row execute function refuse_second()`)
	if err != nil {
		t.Fatal(err)
	}
	pl = f.generate(pl, task("Endpoint", nil, script("make")), task("Button", []int{1}, script("make")), task("Docs", nil, script("make")))
	if pl.Pending != nil || pl.Error == nil || len(pl.Proposals) != 3 || pl.Proposals[0].TaskID == nil || pl.Proposals[1].TaskID != nil {
		t.Fatalf("plan %+v, want the proposals and the first task kept, and the error recorded", pl)
	}

	if _, err := f.DB.Exec(ctx, `drop trigger refuse_second on plan_proposals`); err != nil {
		t.Fatal(err)
	}
	chats := len(f.llm.Chats())
	f.act(pl.ID, "retry", nil).Expect(http.StatusOK, "")
	f.work()
	if pl = f.plan(pl.ID); pl.Status != "done" || len(f.llm.Chats()) != chats {
		t.Fatalf("plan %+v after %d chat requests, want it done without any", pl, len(f.llm.Chats())-chats)
	}
	if n := f.tasks(); n != 3 {
		t.Fatalf("%d tasks, want one per proposal", n)
	}
}

// Generating creates tasks with the creator's GitHub token, so it needs them to push to the repository still.
func TestGenerateNeedsTheCreatorToPush(t *testing.T) {
	f := setup(t)
	pl := f.noReview()
	f.repo.Grant(f.token, "pull") // only counts once the cached permission expires
	if _, err := f.DB.Exec(context.Background(), `update project_members set checked_at = now() - interval '5 minutes 1 second'`); err != nil {
		t.Fatal(err)
	}
	chats := len(f.llm.Chats())
	f.generate(pl, task("Export", nil, script("make")))
	const want = "The plan's creator can no longer push to the repository on GitHub, which creating its tasks needs. Once they can, someone retries."
	if pl = f.plan(pl.ID); pl.Pending != nil || orNull(pl.Error) != want || len(pl.Proposals) != 0 {
		t.Fatalf("plan %+v", pl)
	}
	if n := len(f.llm.Chats()); n != chats || f.tasks() != 0 {
		t.Fatalf("%d chat requests and %d tasks, want none", n-chats, f.tasks())
	}
}

// roundTrip is a RoundTripper from a function.
type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The tests the agent wrote are the card's, and so what automation runs on a ready pull request closing the task.
func TestGeneratedTestsAreWhatARunExecutes(t *testing.T) {
	f := setup(t)
	f.Do("PUT", f.path("/automation"), map[string]any{"decisionProviderId": f.provider, "decisionModel": llmtest.DecisionModel,
		"chatProviderId": f.provider, "chatModel": llmtest.ChatModel}, f.ada).Expect(http.StatusOK, "")
	check := tasktests.Test{Name: "Export answers CSV", Kind: "check", Details: "1. GET /api/export."}
	pl := f.generate(f.noReview(), task("Export", nil, script("go test ./..."), check))
	if pl.Status != "done" {
		t.Fatalf("plan %+v", pl)
	}

	ctx := context.Background()
	f.repo.AddPull("Export", githubtest.Pull{Author: "ada", HeadSHA: "a1", HeadRepo: f.repo.ID,
		Closes: []githubtest.Ref{{RepoID: f.repo.ID, Number: 1}}})
	if err := f.Automation.Poll(ctx, f.project); err != nil {
		t.Fatal(err)
	}
	f.llm.Reply(reply(call("report", map[string]string{"outcome": "pass", "reason": "It answered CSV."})))
	f.Automation.Work(ctx)

	type result struct{ Kind, Text, Status string }
	rows, _ := f.DB.Query(ctx, `select x.kind, x.text, x.status from test_results x join test_runs r on r.id = x.run_id
		join tasks t on t.id = r.task_id where t.id = $1 order by x.position`, *pl.Proposals[0].TaskID)
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[result])
	want := []result{{"script", "`go test ./...`", "passed"}, {"llm", "Export answers CSV\n1. GET /api/export.", "passed"}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("results %+v, %v; want %+v", got, err, want)
	}
	var commands []string
	for _, r := range f.Runner.Requests() {
		if r.Endpoint == "command" {
			commands = append(commands, r.Command)
		}
	}
	if !slices.Equal(commands, []string{"go test ./..."}) {
		t.Fatalf("runner commands %q", commands)
	}
}
