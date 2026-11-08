// Package board serves a project's board: its columns and tasks, task edits and moves, and the sync with GitHub.
package board

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// Board serves the board and task routes under /api/projects/{id}.
type Board struct {
	DB       *pgxpool.Pool
	Accounts *accounts.Accounts // guards every route
	Projects *projects.Projects // checks access to the project first
	GitHub   *github.Client
	// Rules are automation's rules for moves. app.New sets them: automation moves tasks through Board, so board
	// cannot import it. Nil moves freely.
	Rules Rules
}

// Rules add Moves while automated, in AUTOMATION.md, to the moves of a project whose automation is on.
type Rules interface {
	// Moving runs in tx, holding projects.Lock, before task taskID moves from the column with key from to another
	// column with key to, and before GitHub hears of it. byHand is false for MoveToEnd, the API's own moves. A
	// *Refusal stops the move, which the move endpoint answers with 409; any other error fails it.
	Moving(ctx context.Context, tx pgx.Tx, p projects.Project, taskID int64, from, to string, byHand bool) error
}

// Refusal is a move the rules refuse, answered 409 with this error code and message.
type Refusal struct{ Code, Message string }

func (r *Refusal) Error() string { return "board: move refused: " + r.Message }

// Register adds the board routes to mux.
func (b *Board) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects/{id}/board", b.Accounts.Require(b.show))
	mux.HandleFunc("POST /api/projects/{id}/tasks", b.Accounts.Require(b.createTask))
	mux.HandleFunc("GET /api/projects/{id}/tasks/{taskID}", b.Accounts.Require(b.getTask))
	mux.HandleFunc("PATCH /api/projects/{id}/tasks/{taskID}", b.Accounts.Require(b.editTask))
	mux.HandleFunc("POST /api/projects/{id}/tasks/{taskID}/move", b.Accounts.Require(b.move))
}

type column struct {
	ID          int64  `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Position    int    `json:"position"`
	ClosesIssue bool   `json:"closesIssue"`
}

// label is the GitHub label that puts an issue in the column.
func (c column) label() string { return "board:" + c.Key }

// columns returns the project's columns in board order.
func columns(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, projectID int64) ([]column, error) {
	rows, _ := db.Query(ctx, `select id, key, name, position, closes_issue from columns where project_id = $1 order by position`, projectID)
	return pgx.CollectRows(rows, pgx.RowToStructByPos[column])
}

type task struct {
	ID          int64   `json:"id"`
	IssueNumber int     `json:"issueNumber"`
	ColumnID    int64   `json:"columnId"`
	Position    float64 `json:"position"`
	Version     int     `json:"version"`
	Title       string  `json:"title"`
	State       string  `json:"state"` // "open" or "closed"
	URL         string  `json:"url"`
	RunStatus   *string `json:"runStatus"` // the latest test run's, nil for none
	// NeedsAttention is why the task waits in Testing after a runner error, nil while clear.
	NeedsAttention *string `json:"needsAttention"`
}

// taskColumns selects, from tasks t, what scanTask reads.
const taskColumns = `t.id, t.issue_number, t.column_id, t.position, t.version, t.title, t.state,
	(select status from test_runs where task_id = t.id order by id desc limit 1), t.needs_attention`

// taskByID selects the task $1 of the project $2.
const taskByID = `select ` + taskColumns + ` from tasks t where t.id = $1 and t.project_id = $2`

// scanTask reads a task of project p from a row of taskColumns, then any further columns into extra.
func (b *Board) scanTask(row pgx.Row, p projects.Project, extra ...any) (task, error) {
	var t task
	if err := row.Scan(append([]any{&t.ID, &t.IssueNumber, &t.ColumnID, &t.Position, &t.Version, &t.Title, &t.State,
		&t.RunStatus, &t.NeedsAttention}, extra...)...); err != nil {
		return task{}, err
	}
	t.URL = fmt.Sprintf("%s/%s/%s/issues/%d", b.GitHub.Web, p.Owner, p.Name, t.IssueNumber)
	return t, nil
}

// taskID returns the {taskID} path value, or 0, which matches no task, when it is not a number.
func taskID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("taskID"), 10, 64)
	return id
}

func taskNotFound(w http.ResponseWriter) {
	httpx.Error(w, http.StatusNotFound, "not_found", "Task not found.")
}

// validTitle trims a task title and checks it fits GitHub's 1 to 256 characters, answering 400 when not.
func validTitle(w http.ResponseWriter, title string) (string, bool) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < 1 || n > 256 {
		httpx.Error(w, http.StatusBadRequest, "invalid_title", "A title needs 1 to 256 characters.")
		return "", false
	}
	return title, true
}

// validBody checks a task body fits GitHub's 65536 characters, answering 400 when not.
func validBody(w http.ResponseWriter, body string) bool {
	if utf8.RuneCountInString(body) > 65536 {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "A body can have at most 65536 characters.")
		return false
	}
	return true
}
