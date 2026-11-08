package automation

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// runExecution is a workflow execution as getRun shows it.
type runExecution struct {
	ID         int64      `json:"id"`
	Workflow   string     `json:"workflow"`
	Status     string     `json:"status"`
	Attempts   int        `json:"attempts"`
	LogTail    string     `json:"logTail"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// runResult is a test result as getRun shows it: its screenshot only as whether it has one.
type runResult struct {
	ID            int64              `json:"id"`
	Position      int                `json:"position"`
	Text          string             `json:"text"`
	Kind          string             `json:"kind"`
	Pinned        bool               `json:"pinned"`
	ExecutionID   *int64             `json:"executionId"` // nil when routing refused the test
	Workflow      *string            `json:"workflow"`
	Probabilities map[string]float64 `json:"probabilities"`
	Status        string             `json:"status"`
	Detail        string             `json:"detail"`
	Transcript    json.RawMessage    `json:"transcript"` // null unless an LLM check made tool calls
	HasScreenshot bool               `json:"hasScreenshot"`
}

// getRun answers one test run with its workflow executions and test results, also once automation is off.
func (a *Automation) getRun(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	ctx := r.Context()
	var out struct {
		run
		Executions []runExecution `json:"executions"`
		Results    []runResult    `json:"results"`
	}
	row, _ := a.DB.Query(ctx, `select `+runColumns+` from test_runs r join tasks t on t.id = r.task_id
		where r.id = $1 and t.project_id = $2`, pathID(r, "runID"), p.ID)
	var err error
	out.run, err = pgx.CollectExactlyOneRow(row, pgx.RowToStructByPos[run])
	if errors.Is(err, pgx.ErrNoRows) {
		runNotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	rows, _ := a.DB.Query(ctx, `
		select id, workflow, status, attempts, log_tail, started_at, finished_at from workflow_executions
		where run_id = $1 order by id`, out.ID)
	if out.Executions, err = pgx.CollectRows(rows, pgx.RowToStructByPos[runExecution]); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	rows, _ = a.DB.Query(ctx, `
		select x.id, x.position, x.text, x.kind, x.pinned, x.execution_id, e.workflow, x.probabilities, x.status, x.detail,
			x.transcript, x.screenshot is not null
		from test_results x left join workflow_executions e on e.id = x.execution_id where x.run_id = $1 order by x.position`, out.ID)
	if out.Results, err = pgx.CollectRows(rows, pgx.RowToStructByPos[runResult]); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
