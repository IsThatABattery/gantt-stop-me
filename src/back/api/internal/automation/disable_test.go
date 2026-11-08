package automation_test

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
)

// run stores a test run with this status on the project's task, and a workflow execution with the same status.
// It returns the run's and the execution's ids.
func (f *fixture) run(project int64, status string) (int64, int64) {
	f.t.Helper()
	var run, execution int64
	ctx := context.Background()
	if err := f.DB.QueryRow(ctx, `insert into test_runs (task_id, status) select id, $2 from tasks where project_id = $1 returning id`,
		project, status).Scan(&run); err != nil {
		f.t.Fatal(err)
	}
	if err := f.DB.QueryRow(ctx, `
		insert into workflow_executions (run_id, project_id, workflow, spec, status) values ($1, $2, 'default', '{}', $3) returning id`,
		run, project, status).Scan(&execution); err != nil {
		f.t.Fatal(err)
	}
	return run, execution
}

// statuses returns the status of each row of table, test_runs or workflow_executions, by id.
func (f *fixture) statuses(table string) map[int64]string {
	f.t.Helper()
	all := map[int64]string{}
	rows, _ := f.DB.Query(context.Background(), `select id, status from `+table)
	var id int64
	var status string
	if _, err := pgx.ForEachRow(rows, []any{&id, &status}, func() error { all[id] = status; return nil }); err != nil {
		f.t.Fatal(err)
	}
	return all
}

// projectIDs returns the project_id of each row of table, in order.
func (f *fixture) projectIDs(table string) []int64 {
	f.t.Helper()
	rows, _ := f.DB.Query(context.Background(), `select project_id from `+table+` order by project_id`)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		f.t.Fatal(err)
	}
	return ids
}

func TestDisable(t *testing.T) {
	f := setup(t)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	// A second project of ada's, which turning off the first must leave alone.
	otherRepo := f.GitHub.AddRepo("octo", "other")
	otherRepo.Grant(f.token, "push")
	otherRepo.AddIssue("Other")
	other := f.AddProject(f.ada, "octo", "other")
	f.exec(`insert into project_automation (project_id, owner_id, decision_provider, decision_model, chat_provider, chat_model)
		values ($1, $2, $3, 'luna', $3, 'chat')`, other, f.adaID, f.provider)
	f.exec(`insert into pull_requests (project_id, number, head_sha, author_id, draft) values ($1, 7, 'abc', 1, false), ($2, 7, 'abc', 1, false)`,
		f.project, other)

	queuedRun, queuedExec := f.run(f.project, "queued")
	runningRun, runningExec := f.run(f.project, "running")
	f.run(f.project, "passed")
	f.run(f.project, "error")
	f.run(other, "running")
	wantRuns, wantExecs := f.statuses("test_runs"), f.statuses("workflow_executions")
	wantRuns[queuedRun], wantRuns[runningRun] = "cancelled", "cancelled"
	wantExecs[queuedExec], wantExecs[runningExec] = "cancelled", "cancelled"

	f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "")

	if got := f.statuses("test_runs"); !maps.Equal(got, wantRuns) {
		t.Errorf("test runs = %v, want %v", got, wantRuns)
	}
	if got := f.statuses("workflow_executions"); !maps.Equal(got, wantExecs) {
		t.Errorf("workflow executions = %v, want %v", got, wantExecs)
	}
	var reason *string
	f.DB.QueryRow(context.Background(), `select reason from test_runs where id = $1`, runningRun).Scan(&reason)
	if reason == nil || *reason != "Automation was turned off." {
		t.Errorf("cancelled run's reason = %v", reason)
	}
	for _, table := range []string{"project_automation", "pull_requests"} {
		if got := f.projectIDs(table); !slices.Equal(got, []int64{other}) {
			t.Errorf("%s rows of projects %v, want only %d's", table, got, other)
		}
	}
	if s := f.Do("GET", f.path("/automation"), nil, f.ada); string(s.Body) != "{\"enabled\":false}\n" {
		t.Errorf("GET after DELETE = %s", s.Body)
	}
}

func TestDisableWhenOff(t *testing.T) {
	f := setup(t)
	f.Do("DELETE", f.path("/automation"), nil, f.ada).Expect(http.StatusNoContent, "")
}

func TestDisableNeedsWriteAccess(t *testing.T) {
	f := setup(t)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	bob, _, _ := f.member("bob", "pull")
	eve, _, _ := f.member("eve", "")
	f.Do("DELETE", f.path("/automation"), nil, bob).Expect(http.StatusForbidden, "forbidden")
	f.Do("DELETE", f.path("/automation"), nil, eve).Expect(http.StatusNotFound, "not_found")
	f.Do("DELETE", f.path("/automation"), nil).Expect(http.StatusUnauthorized, "unauthorized")
	f.Do("DELETE", "/api/projects/999/automation", nil, f.ada).Expect(http.StatusNotFound, "not_found")
	if s := f.show(f.ada); !s.Enabled {
		t.Fatal("a refused DELETE turned automation off")
	}
}
