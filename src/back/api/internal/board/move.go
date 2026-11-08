package board

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// minGap is the smallest gap between neighbours a midpoint may split before the column is renumbered.
const minGap = 1e-9

// move puts a task in a column before another task, or at its end, and writes a column change through to GitHub
// first. It holds the project's lock throughout, so nothing else places tasks in the meantime. Moves waiting for
// the lock queue in projects.Serialize, before they take a DB connection.
func (b *Board) move(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		ColumnID     int64  `json:"columnId"`
		BeforeTaskID *int64 `json:"beforeTaskId"` // nil for the end of the column
		Version      int    `json:"version"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	unlock, err := projects.Serialize(r.Context(), p.ID)
	if err != nil {
		return // the client gave up waiting
	}
	defer unlock()
	// Once GitHub has the move, finish saving it even if the browser goes away.
	ctx := context.WithoutCancel(r.Context())
	tx, err := b.DB.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer tx.Rollback(ctx)
	if err := projects.Lock(ctx, tx, p.ID); err != nil {
		httpx.Internal(w, r, err)
		return
	}

	t, err := b.scanTask(tx.QueryRow(ctx, taskByID, taskID(r), p.ID), p)
	if errors.Is(err, pgx.ErrNoRows) {
		taskNotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if in.Version != t.Version {
		httpx.JSON(w, http.StatusConflict, map[string]any{
			"error": httpx.ErrorBody{Code: "conflict", Message: "This task changed since you loaded it. It is shown where it is now."},
			"task":  t,
		})
		return
	}
	to, err := columnByID(ctx, tx, p.ID, in.ColumnID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusBadRequest, "invalid_column", "That column is not on this board.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	from := t.ColumnID
	t, err = b.place(ctx, tx, token, p, t, to, in.BeforeTaskID, true)
	if err == nil && to.Key == "in-progress" && from != to.ID && t.Branch == nil {
		t.Branch = b.branch(ctx, tx, token, p, t)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	var refusal *Refusal
	switch {
	case errors.As(err, &refusal):
		httpx.Error(w, http.StatusConflict, refusal.Code, refusal.Message)
	case errors.Is(err, errInvalidBefore):
		httpx.Error(w, http.StatusBadRequest, "invalid_before_task", "beforeTaskId must be another task in the target column.")
	case errors.As(err, new(GitHubError)), errors.As(err, new(*github.Error)): // the latter from the rules
		github.Fail(w, r, err)
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		httpx.JSON(w, http.StatusOK, t)
	}
}

// MoveToEnd moves the project's task to the end of the column with key, as the move endpoint would: a column change
// goes to GitHub with token first, and a failed GitHub write is undone as far as GitHub lets it and comes back as
// GitHub's error. Only the rules that hold for the API's own moves apply. Call it in tx holding projects.Lock, and
// commit tx only when it succeeds.
func (b *Board) MoveToEnd(ctx context.Context, tx pgx.Tx, token string, p projects.Project, taskID int64, key string) error {
	t, err := b.scanTask(tx.QueryRow(ctx, taskByID, taskID, p.ID), p)
	if err != nil {
		return err
	}
	var to column
	err = tx.QueryRow(ctx, `select id, key, name, position, closes_issue from columns where project_id = $1 and key = $2`, p.ID, key).
		Scan(&to.ID, &to.Key, &to.Name, &to.Position, &to.ClosesIssue)
	if err != nil {
		return err
	}
	_, err = b.place(ctx, tx, token, p, t, to, nil, false)
	return err
}

// errInvalidBefore means the task to place a task before is not another task in the target column.
var errInvalidBefore = errors.New("board: before is not another task in the column")

// GitHubError marks a failed GitHub write, of place or CreateTask, apart from a database error, so endpoints answer it
// with github.Fail.
type GitHubError struct{ error }

func (e GitHubError) Unwrap() error { return e.error }

// place puts task t in column to before the task before, or at the end when before is nil, writing a column change
// through to GitHub with token first, and returns the task as saved. A column change follows b.Rules first, as a
// person's move when byHand. Call it in tx holding projects.Lock.
func (b *Board) place(ctx context.Context, tx pgx.Tx, token string, p projects.Project, t task, to column, before *int64, byHand bool) (task, error) {
	from, err := columnByID(ctx, tx, p.ID, t.ColumnID)
	if err != nil {
		return task{}, err
	}
	pos, ok, err := position(ctx, tx, t.ID, to.ID, before)
	if err != nil {
		return task{}, err
	}
	if !ok {
		return task{}, errInvalidBefore
	}
	if to.ID != from.ID && b.Rules != nil {
		if err := b.Rules.Moving(ctx, tx, token, p, t.ID, from.Key, to.Key, byHand); err != nil {
			return task{}, err
		}
	}

	// nil keeps the stored labels: a reorder writes nothing to GitHub.
	state, boardLabels, labels := t.State, []string(nil), []github.Label(nil)
	if to.ID != from.ID {
		if state, labels, err = b.relabel(ctx, token, p, t, from, to); err != nil {
			return task{}, GitHubError{err}
		}
		boardLabels = []string{to.label()}
	}
	// clock_timestamp, not now(): a sync that began fetching before GitHub had this move must see it as newer.
	return b.scanTask(tx.QueryRow(ctx, `
		update tasks t set column_id = $2, position = $3, version = version + 1, state = $4, updated_at = clock_timestamp(),
			board_labels = coalesce($5, t.board_labels), labels = coalesce($6, t.labels)
		where t.id = $1 returning `+taskColumns, t.ID, to.ID, pos, state, boardLabels, labels), p)
}

// columnByID returns the project's column id.
func columnByID(ctx context.Context, tx pgx.Tx, projectID, id int64) (column, error) {
	var c column
	err := tx.QueryRow(ctx, `select id, key, name, position, closes_issue from columns where id = $1 and project_id = $2`, id, projectID).
		Scan(&c.ID, &c.Key, &c.Name, &c.Position, &c.ClosesIssue)
	return c, err
}

// position returns where the task lands in the column: midway between before and the task above it,
// or after the last task when before is nil. ok is false when before is not another task in the column.
func position(ctx context.Context, tx pgx.Tx, taskID, columnID int64, before *int64) (pos float64, ok bool, err error) {
	if before == nil {
		err = tx.QueryRow(ctx, `select coalesce(max(position), 0) + 1 from tasks where column_id = $1 and id <> $2`, columnID, taskID).Scan(&pos)
		return pos, err == nil, err
	}
	if *before == taskID {
		return 0, false, nil
	}
	var at float64
	var above *float64
	err = tx.QueryRow(ctx, `
		select b.position, (select max(position) from tasks where column_id = $2 and id <> $3 and position < b.position)
		from tasks b where b.id = $1 and b.column_id = $2`, *before, columnID, taskID).Scan(&at, &above)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, err
	case above == nil:
		return at - 1, true, nil
	case at-*above < minGap:
		// Renumbering leaves gaps of 1, so the second try splits one.
		_, err = tx.Exec(ctx, `
			update tasks t set position = n.rn
			from (select id, row_number() over (order by position, id) as rn from tasks where column_id = $1) n
			where t.id = n.id`, columnID)
		if err != nil {
			return 0, false, err
		}
		return position(ctx, tx, taskID, columnID, before)
	}
	return (at + *above) / 2, true, nil
}

// relabel replaces the issue's board labels with the to column's, closes it on entering a closing column
// and reopens it on leaving one. It returns the issue's state and its labels as GitHub last answered them.
// When a step fails after GitHub took the new label, it puts from's label back, as far as GitHub lets it,
// so GitHub matches the board the failed move leaves as it was.
func (b *Board) relabel(ctx context.Context, token string, p projects.Project, t task, from, to column) (state string, labels []github.Label, err error) {
	gh := b.GitHub
	all, err := gh.AddLabels(ctx, token, p.RepoID, t.IssueNumber, []string{to.label()})
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if err != nil {
			gh.RemoveLabel(ctx, token, p.RepoID, t.IssueNumber, to.label())
			gh.AddLabels(ctx, token, p.RepoID, t.IssueNumber, []string{from.label()})
		}
	}()
	labels = []github.Label{} // not nil, which would keep the stored labels
	for _, l := range all {
		// Every other board label, not just from's: one a sync move left behind would pull the task back.
		if name := strings.ToLower(l.Name); strings.HasPrefix(name, "board:") && name != to.label() {
			if err := gh.RemoveLabel(ctx, token, p.RepoID, t.IssueNumber, l.Name); err != nil {
				return "", nil, err
			}
			continue
		}
		labels = append(labels, l)
	}
	state = t.State
	switch {
	case to.ClosesIssue && !from.ClosesIssue:
		state = "closed"
	case from.ClosesIssue && !to.ClosesIssue:
		state = "open"
	default:
		return state, labels, nil
	}
	is, err := gh.UpdateIssue(ctx, token, p.RepoID, t.IssueNumber, map[string]any{"state": state})
	return state, projects.Labels(is), err
}

// branch makes the task's branch on GitHub, off the default branch's head, and records it. A failure leaves the task
// without one, logged, rather than refusing the move: the branch is a convenience, the move is what was asked.
func (b *Board) branch(ctx context.Context, tx pgx.Tx, token string, p projects.Project, t task) *string {
	name := BranchName(t.IssueNumber, t.Title)
	if _, err := b.GitHub.CreateBranch(ctx, token, p.RepoID, name); err != nil {
		log.Printf("branch for task %d: %v", t.ID, err)
		return nil
	}
	if _, err := tx.Exec(ctx, `update tasks set branch = $2 where id = $1`, t.ID, name); err != nil {
		log.Printf("branch for task %d: %v", t.ID, err)
		return nil
	}
	return &name
}

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

// BranchName is a task's branch: feature/, its issue number, and its title in at most 40 lowercase letters, digits
// and hyphens, cut at a word's end.
func BranchName(issue int, title string) string {
	slug := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(slug) > 40 { // at a word's end
		if slug = slug[:41]; strings.LastIndex(slug, "-") > 0 {
			slug = slug[:strings.LastIndex(slug, "-")]
		}
		slug = strings.TrimRight(slug, "-")
	}
	if slug == "" {
		return fmt.Sprintf("feature/%d", issue)
	}
	return fmt.Sprintf("feature/%d-%s", issue, slug)
}
