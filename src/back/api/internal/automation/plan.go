package automation

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/llm"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/tasktests"
)

// planAll plans every queued test run of an automated, unpaused project that has no test results yet, oldest first.
func (a *Automation) planAll(ctx context.Context) {
	rows, _ := a.DB.Query(ctx, `
		select r.id from test_runs r join tasks t on t.id = r.task_id
		join project_automation pa on pa.project_id = t.project_id and pa.paused_reason is null
		where r.status = 'queued' and not exists (select from test_results x where x.run_id = r.id)
		order by r.id`)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	report(ctx, "list test runs to plan", err)
	for _, id := range ids {
		report(ctx, fmt.Sprintf("plan test run %d", id), a.plan(ctx, id))
	}
}

// plan works out a queued test run's workflow executions, outside the project's lock: it reads the issue fresh from
// GitHub with the owner's credential, checks its author can push, takes the card's tests from it, reads
// .gantt/workflows.yml at the run's head, and routes each of the card's tests. Then, if the run is still queued, it
// saves in one transaction a queued workflow execution per workflow that has tests, and a test result per test. A
// run that cannot be planned, or whose tests all failed to route, ends at once as a runner error. A paused owner, or
// a GitHub failure a retry may mend, leaves it queued.
func (a *Automation) plan(ctx context.Context, runID int64) error {
	var p projects.Project
	var taskID int64
	var issue, pr int
	var head string
	var synced *string // the card's tests_synced before the issue is read
	err := a.DB.QueryRow(ctx, `
		select p.id, p.github_repo_id, p.owner, p.name, t.id, t.issue_number, coalesce(r.pr_number, 0), coalesce(r.head_sha, ''),
			t.tests_synced
		from test_runs r join tasks t on t.id = r.task_id join projects p on p.id = t.project_id where r.id = $1`, runID).
		Scan(&p.ID, &p.RepoID, &p.Owner, &p.Name, &taskID, &issue, &pr, &head, &synced)
	if err != nil {
		return err
	}
	token, err := a.OwnerToken(ctx, p.ID)
	if err != nil {
		return err
	}
	if pr == 0 { // defensive: every run without a pull request starts ended, by startRun
		return a.finish(ctx, runID, "No linked pull request.")
	}
	is, err := a.GitHub.Issue(ctx, token, p.RepoID, issue)
	var trusted bool
	if err == nil {
		trusted, err = a.canPush(ctx, token, p, is.User.Login)
	}
	if err != nil {
		return a.gitHubFailed(ctx, p.ID, runID, token, "read the issue", err)
	}
	// Its tests run with the workflow's caches, which later runs trust. Only the author and those with write access
	// can edit an issue's body, so its author having write access means everyone who wrote it had.
	if !trusted {
		return a.finish(ctx, runID, "The issue was opened by someone without write access to the repository, so its tests do not run.")
	}
	var tests []tasktests.Test
	var testsError *string
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		// A card that took or wrote a section since the read, by a tests edit or a sync, is newer than the issue read here.
		var changed bool
		err := tx.QueryRow(ctx, `select tests_synced is distinct from $2 from tasks where id = $1 for update`, taskID, synced).Scan(&changed)
		if err == nil && !changed {
			_, err = projects.TakeTests(ctx, tx, taskID, is.Body)
		}
		if err == nil {
			err = tx.QueryRow(ctx, `select tests, tests_error from tasks where id = $1`, taskID).Scan(&tests, &testsError)
		}
		return err
	})
	switch {
	case err != nil:
		return err
	case testsError != nil: // the card's tests are not what the issue shows
		return a.finish(ctx, runID, *testsError)
	case len(tests) == 0:
		return a.finish(ctx, runID, "The task has no tests.")
	}
	ws := DefaultWorkflows()
	file, err := a.GitHub.WorkflowFile(ctx, token, p.RepoID, head)
	switch ghErr := (*github.Error)(nil); {
	case errors.As(err, &ghErr) && ghErr.Status == http.StatusNotFound: // no file at that commit
	case err != nil:
		return a.gitHubFailed(ctx, p.ID, runID, token, "read .gantt/workflows.yml", err)
	default:
		if ws, err = ParseWorkflows(file); err != nil {
			return a.finish(ctx, runID, ".gantt/workflows.yml: "+err.Error())
		}
	}
	routes, err := a.route(ctx, p.ID, is.Title, ws, tests)
	if err != nil {
		return a.finish(ctx, runID, "The decision model could not route the tests: "+err.Error())
	}

	executions := 0
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		var queued bool
		err := tx.QueryRow(ctx, `
			select status = 'queued' and not exists (select from test_results where run_id = $1) from test_runs where id = $1 for update`,
			runID).Scan(&queued)
		if err != nil || !queued {
			return err // cancelled or planned meanwhile
		}
		ids := map[string]int64{}
		for _, w := range ws {
			if !slices.ContainsFunc(routes, func(r route) bool { return r.workflow == w.Name }) {
				continue
			}
			spec, err := json.Marshal(struct {
				Project int64 `json:"project"`
				Workflow
			}{p.ID, w})
			if err != nil {
				return err
			}
			var id int64
			if err := tx.QueryRow(ctx, `
				insert into workflow_executions (run_id, project_id, workflow, spec, status, timeout)
				values ($1, $2, $3, $4, 'queued', $5) returning id`, runID, p.ID, w.Name, spec, w.Timeout).Scan(&id); err != nil {
				return err
			}
			ids[w.Name] = id
		}
		executions = len(ids)
		for i, t := range tests {
			r := routes[i]
			var execution *int64
			status := "error"
			if id, ok := ids[r.workflow]; ok {
				execution, status = &id, "pending"
			}
			kind := "llm" // a check, as results name it
			if t.Kind == "script" {
				kind = "script"
			}
			if _, err := tx.Exec(ctx, `
				insert into test_results (run_id, execution_id, position, text, kind, command, pinned, probabilities, status, detail)
				values ($1, $2, $3, $4, $5, nullif($6, ''), $7, $8, $9, $10)`,
				runID, execution, i+1, testText(t), kind, t.Command, t.Workflow != nil, r.probabilities, status, r.refusal); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil || executions > 0 {
		return err
	}
	return a.finish(ctx, runID, "")
}

// gitHubFailed handles GitHub failing a read that run runID's plan needs: GitHub refusing the owner's credential
// pauses the project, and a failure a retry may mend leaves the run queued for the next pass. Any other refusal,
// such as a 404 for an issue deleted meanwhile, ends the run as a runner error.
func (a *Automation) gitHubFailed(ctx context.Context, projectID, runID int64, token, what string, err error) error {
	if err = a.PauseIfRejected(ctx, projectID, token, err); !lasting(err) {
		return err
	}
	return a.finish(ctx, runID, fmt.Sprintf("GitHub refused to %s: %v.", what, err))
}

// route is the workflow one test goes to.
type route struct {
	workflow      string             // "" when the test does not run
	probabilities map[string]float64 // the decision model's, by workflow name
	refusal       string             // why the test does not run: its runner error
}

// route gives each test a workflow of ws, as Routing says: a pinned test goes to its pin, every test goes to the only
// workflow when there is one, and otherwise the owner's decision model picks for every unpinned test in one call. A
// pin naming no workflow, a top probability under 0.5, a refusal and a missing answer are each a runner error for
// that test. The error is the decisions call failing.
func (a *Automation) route(ctx context.Context, projectID int64, title string, ws []Workflow, tests []tasktests.Test) ([]route, error) {
	names := map[string]bool{}
	for _, w := range ws {
		names[w.Name] = true
	}
	routes := make([]route, len(tests))
	var ask []int // the positions of the tests the decision model picks for
	for i, t := range tests {
		routes[i].probabilities = map[string]float64{}
		switch {
		case t.Workflow != nil && names[*t.Workflow]:
			routes[i].workflow = *t.Workflow
		case t.Workflow != nil:
			routes[i].refusal = fmt.Sprintf("Test %d is pinned to [%s], which names no workflow.", i+1, *t.Workflow)
		case len(ws) == 1:
			routes[i].workflow = ws[0].Name
		default:
			ask = append(ask, i+1)
		}
	}
	if len(ask) == 0 {
		return routes, nil
	}
	answers, err := a.decide(ctx, projectID, title, ws, tests, ask)
	if err != nil {
		return nil, err
	}
	for _, pos := range ask {
		r := &routes[pos-1]
		for _, p := range answers[question(pos)].Probabilities {
			if names[p.Value] {
				r.probabilities[p.Value] = p.Probability
			}
		}
		best := slices.SortedFunc(maps.Keys(r.probabilities), func(x, y string) int {
			return cmp.Or(cmp.Compare(r.probabilities[y], r.probabilities[x]), cmp.Compare(x, y))
		})
		if len(best) > 0 && r.probabilities[best[0]] >= 0.5 {
			r.workflow = best[0]
			continue
		}
		guesses := []string{}
		for _, name := range best[:min(3, len(best))] {
			guesses = append(guesses, fmt.Sprintf("%s %.0f%%", name, 100*r.probabilities[name]))
		}
		r.refusal = fmt.Sprintf("Test %d matches no workflow well enough to run. Best guesses: %s. Pin it to one with [name].",
			pos, cmp.Or(strings.Join(guesses, ", "), "none"))
	}
	return routes, nil
}

// decide asks the owner's decision model, in one call, which of ws can run each of tests at the positions ask, and
// returns its answers by question name.
func (a *Automation) decide(ctx context.Context, projectID int64, title string, ws []Workflow, tests []tasktests.Test, ask []int) (map[string]llm.Answer, error) {
	var baseURL, model, instructions string
	var sealed []byte
	err := a.DB.QueryRow(ctx, `
		select pv.base_url, pv.key_enc, pa.decision_model, pa.decision_instructions
		from project_automation pa join providers pv on pv.id = pa.decision_provider where pa.project_id = $1`, projectID).
		Scan(&baseURL, &sealed, &model, &instructions)
	if err != nil {
		return nil, err
	}
	key, err := a.Secrets.Open(sealed)
	if err != nil {
		return nil, err
	}
	var in strings.Builder
	fmt.Fprintf(&in, "Issue: %s\n\nWorkflows:\n", title)
	choices := make([]llm.Choice, len(ws))
	for i, w := range ws {
		fmt.Fprintf(&in, "- %s: %s\n", w.Name, w.Description)
		choices[i] = llm.Choice{Value: w.Name, Description: w.Description}
	}
	in.WriteString("\nTests:")
	questions := make([]llm.Question, len(ask))
	for i, pos := range ask {
		fmt.Fprintf(&in, "\n%d. %s", pos, strings.ReplaceAll(testText(tests[pos-1]), "\n", "\n   "))
		questions[i] = llm.Question{Name: question(pos), Choices: choices, Instructions: fmt.Sprintf("Which workflow can run test %d?", pos) +
			llm.Extra("the project's automation owner", instructions)}
	}
	c := &llm.Client{BaseURL: baseURL, Key: string(key), HTTP: a.LLM}
	return c.Decide(ctx, model, in.String(), questions)
}

// question is the name of the decisions question about the test at position pos.
func question(pos int) string { return fmt.Sprintf("test_%d", pos) }

// testText is test t as the issue's Tests section shows it, past its bullet, checkbox and pin: its line, then any
// block under it. Its result, its comment and the decision model read it.
func testText(t tasktests.Test) string {
	t.Workflow = nil
	item := strings.TrimPrefix(tasktests.Render([]tasktests.Test{t}), "## Tests\n- [ ] ")
	return strings.ReplaceAll(strings.TrimSuffix(item, "\n"), "\n  ", "\n")
}
