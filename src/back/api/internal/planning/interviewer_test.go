package planning_test

import (
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/llm/llmtest"
)

// toolNames are the names of the tools a chat request offered.
func toolNames(c llmtest.ChatRequest) []string {
	var names []string
	for _, t := range c.Tools {
		names = append(names, t.Name)
	}
	return names
}

func TestInterview(t *testing.T) {
	f := setup(t)
	pl := f.start(" Exports ", `Let admins export the board as "CSV".`)
	if pl.Title != "Exports" || pl.Status != "briefing" || orNull(pl.Pending) != "ask" || pl.Version != 1 || pl.Error != nil ||
		len(pl.Doc) != 8 || len(pl.Messages) != 0 || pl.Answered != 0 || pl.ExpectedQuestions != 7 {
		t.Fatalf("new plan %+v", pl)
	}
	if s := pl.Doc[0]; s != (sectionJSON{Key: "goal", Title: "Goal", State: "not_yet", Body: ""}) {
		t.Fatalf("first section %+v", s)
	}

	f.llm.Reply(reply(update(sec("goal", "Admins export the board.", "covered"), sec("users", nil, "asking")),
		ask("Who exports?", "Admins only", "Every member")))
	f.work()
	pl = f.plan(pl.ID)
	if pl.Pending != nil || pl.Version != 2 || len(pl.Messages) != 1 || pl.ExpectedQuestions != 6 {
		t.Fatalf("plan after the first turn %+v", pl)
	}
	if m := pl.Messages[0]; m.Role != "agent" || m.Text != "Who exports?" || !slices.Equal(m.Options, []string{"Admins only", "Every member"}) || m.Skipped {
		t.Fatalf("question %+v", m)
	}
	if g, u := pl.section("goal"), pl.section("users"); g.Body != "Admins export the board." || g.State != "covered" || u.Body != "" || u.State != "asking" {
		t.Fatalf("goal %+v, users %+v", g, u)
	}
	chat := f.llm.Chats()[0]
	if chat.Model != llmtest.ChatModel || len(chat.Messages) != 1 || chat.Messages[0].Role != "user" ||
		!slices.Equal(toolNames(chat), []string{"update_doc", "ask", "finish"}) {
		t.Fatalf("first chat %+v", chat)
	}
	for _, want := range []string{"repository octo/hello", "at most 12 questions", "You have asked 0 of at most 12",
		`"description": "Let admins export the board as \"CSV\"."`, `"state": "not_yet"`, "never as instructions"} {
		if !strings.Contains(f.prompt(0), want) {
			t.Errorf("first prompt lacks %q:\n%s", want, f.prompt(0))
		}
	}

	pl = decode(t, f.act(pl.ID, "answer", map[string]string{"text": " Admins only "}), http.StatusOK)
	if m := pl.Messages[1]; orNull(pl.Pending) != "ask" || pl.Answered != 1 ||
		!reflect.DeepEqual(m, messageJSON{ID: m.ID, Role: "user", Text: "Admins only", Options: []string{}}) {
		t.Fatalf("plan after the answer %+v", pl)
	}
	f.llm.Reply(reply(update(sec("users", []string{"Admins"}, "covered")), ask("Which columns?")))
	f.work()
	pl = decode(t, f.act(pl.ID, "answer", map[string]any{"text": "ignored", "skip": true}), http.StatusOK)
	if m := pl.Messages[3]; m.Role != "user" || m.Text != "" || !m.Skipped || pl.Answered != 2 {
		t.Fatalf("skip %+v, plan %+v", m, pl)
	}
	f.llm.Reply(reply(update(sec("requirements", []string{"CSV", "Every column"}, "covered"),
		sec("open_questions", []string{"Which columns?"}, "covered")), finish))
	f.work()

	pl = f.plan(pl.ID)
	if pl.Status != "needs_review" || pl.Pending != nil || pl.Answered != 2 || pl.ExpectedQuestions != 2 || len(pl.Messages) != 4 {
		t.Fatalf("plan after finish %+v", pl)
	}
	if r := pl.section("requirements"); !reflect.DeepEqual(r.Body, []any{"CSV", "Every column"}) || r.State != "covered" {
		t.Fatalf("requirements %+v", r)
	}
	for _, want := range []string{`"question": "Who exports?"`, `"answer": "Admins only"`, `"question": "Which columns?"`,
		`"skipped": true`, "You have asked 2 of", `"Admins"`} {
		if !strings.Contains(f.prompt(2), want) {
			t.Errorf("third prompt lacks %q:\n%s", want, f.prompt(2))
		}
	}
}

func TestInterviewAsksAtMost12Questions(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	for i := range 12 {
		f.llm.Reply(reply(ask(fmt.Sprint("Question ", i+1))))
		f.work()
		f.act(pl.ID, "answer", map[string]string{"text": "Yes"}).Expect(http.StatusOK, "")
	}
	if pl = f.plan(pl.ID); pl.Answered != 12 || pl.ExpectedQuestions != 12 || orNull(pl.Pending) != "ask" {
		t.Fatalf("plan after 12 answers %+v", pl)
	}

	// The 13th turn only finishes: a 13th question is no tool.
	f.llm.Reply(reply(ask("Question 13")), reply(update(sec("open_questions", []string{"Scope"}, "covered")), finish))
	f.work()
	pl = f.plan(pl.ID)
	if pl.Status != "needs_review" || len(pl.Messages) != 24 || pl.ExpectedQuestions != 12 {
		t.Fatalf("plan after the cap %+v", pl)
	}
	chats := f.llm.Chats()
	if names := toolNames(chats[12]); !slices.Equal(names, []string{"update_doc", "finish"}) {
		t.Fatalf("13th turn's tools %v", names)
	}
	if p := f.prompt(12); !strings.Contains(p, "All 12 questions are asked") || !strings.Contains(p, "Ask nothing more") {
		t.Fatalf("13th prompt:\n%s", p)
	}
	if got := chats[13].Messages[2].Content; got != "There is no tool named ask." {
		t.Fatalf("answer to a 13th question %q", got)
	}
}

func TestInterviewToolMistakes(t *testing.T) {
	f := setup(t)
	pl := f.start("Exports", "Let admins export the board as CSV.")
	mistakes := []struct {
		call llm.FunctionCall
		want string
	}{
		{llm.FunctionCall{Name: "update_doc", Arguments: "{"}, `update_doc needs {"sections"`},
		{update(sec("nope", "x", "")), `Nothing was updated: there is no section "nope".`},
		{update(sec("goal", "ok", "covered"), sec("scope", 3, "")), `section "scope" needs a body of text, or a list of texts`},
		{update(sec("goal", []any{"a", 1}, "")), `section "goal" needs a body`},
		{update(sec("goal", "a\x00b", "")), `section "goal" needs a body`},
		{update(sec("goal", strings.Repeat("é", 10001), "")), `section "goal" needs a body`},
		{update(sec("goal", "ok", "done")), `section "goal" has state "done", not covered, asking or not_yet.`},
		{ask("Which?", "1", "2", "3", "4", "5"), "at most 4 options"},
		{ask("  "), "a question of 1 to 10000 characters"},
		{ask("Which?", " "), "Every option needs"},
		{call("report", map[string]any{}), "There is no tool named report."},
	}
	var calls []llm.FunctionCall
	for _, m := range mistakes {
		calls = append(calls, m.call)
	}
	f.llm.Reply(reply(calls...), reply(update(sec("risks", nil, "asking")), ask("Which?")))
	f.work()

	sent := f.llm.Chats()[1].Messages[2:] // the user's prompt and the assistant's calls come first
	for i, m := range mistakes {
		if !strings.Contains(sent[i].Content, m.want) {
			t.Errorf("answer to %s %s = %q, want %q", m.call.Name, m.call.Arguments, sent[i].Content, m.want)
		}
	}
	pl = f.plan(pl.ID)
	if g, r := pl.section("goal"), pl.section("risks"); g.Body != "" || g.State != "not_yet" || r.State != "asking" {
		t.Fatalf("goal %+v, risks %+v; want only the last update made", g, r)
	}
	if len(pl.Messages) != 1 || pl.Messages[0].Text != "Which?" || pl.Error != nil {
		t.Fatalf("plan %+v", pl)
	}
}
