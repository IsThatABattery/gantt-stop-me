package projects

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
)

// columns are every board's columns, in order. Each has a board:<key> label on GitHub in color.
var columns = []struct {
	key, name, color string
	closesIssue      bool
}{
	{"backlog", "Backlog", "c5def5", false},
	{"in-progress", "In Progress", "fbca04", false},
	{"testing", "Testing", "d4c5f9", false},
	{"user-check", "User Check", "f9d0c4", false},
	{"closed", "Closed", "0e8a16", true},
}

// initialize creates the project's columns, its board labels on GitHub and a task for each open issue,
// skipping whatever an earlier attempt made, and marks the project initialized last.
// It answers the request itself and returns false when it fails.
func (p *Projects) initialize(w http.ResponseWriter, r *http.Request, token string, pr Project) bool {
	ctx := r.Context()
	for i, c := range columns {
		_, err := p.DB.Exec(ctx, `
			insert into columns (project_id, key, name, position, closes_issue) values ($1, $2, $3, $4, $5)
			on conflict do nothing`, pr.ID, c.key, c.name, i, c.closesIssue)
		if err != nil {
			httpx.Internal(w, r, err)
			return false
		}
	}
	for _, c := range columns {
		if err := p.GitHub.CreateLabel(ctx, token, pr.RepoID, "board:"+c.key, c.color); err != nil {
			github.Fail(w, r, err)
			return false
		}
	}
	// The first sync re-reads changes from synced_at on, so it is the time the fetch began, as sync records it.
	var started time.Time
	if err := p.DB.QueryRow(ctx, `select now()`).Scan(&started); err != nil {
		httpx.Internal(w, r, err)
		return false
	}
	var issues []github.Issue
	query := url.Values{"state": {"open"}, "sort": {"created"}, "direction": {"asc"}, "per_page": {"100"}}
	if _, err := p.GitHub.Issues(ctx, token, pr.RepoID, query, "", func(page []github.Issue) bool {
		issues = append(issues, page...)
		return true
	}); err != nil {
		github.Fail(w, r, err)
		return false
	}

	unlock, err := Serialize(ctx, pr.ID)
	if err != nil {
		return false // the client gave up waiting
	}
	defer unlock()
	err = pgx.BeginFunc(ctx, p.DB, func(tx pgx.Tx) error {
		if err := Lock(ctx, tx, pr.ID); err != nil {
			return err
		}
		for _, is := range issues {
			if is.PullRequest != nil {
				continue
			}
			if err := AddTask(ctx, tx, pr.ID, is); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `update projects set synced_at = $2 where id = $1`, pr.ID, started)
		return err
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return false
	}
	return true
}

// AddTask adds an open issue as a task at the end of the first column, in board order, whose board:<key> label
// it carries, else Backlog. Closing columns are skipped: an open issue does not belong in one. The task records
// those labels as seen, so a sync moves it only for a label added later.
// An issue that already has a task keeps it as it is. Call it holding Lock.
func AddTask(ctx context.Context, tx pgx.Tx, projectID int64, is github.Issue) error {
	labels := BoardLabels(is)
	_, err := tx.Exec(ctx, `
		insert into tasks (project_id, issue_number, github_issue_id, column_id, position, title, state, board_labels, body, labels)
		select $1, $2, $3, c.id, coalesce((select max(position) from tasks where column_id = c.id), 0) + 1, $4, 'open', $5, $6, $7
		from columns c
		where c.project_id = $1 and c.key = coalesce(
			(select key from columns
			 where project_id = $1 and not closes_issue and 'board:' || key = any($5)
			 order by position limit 1),
			'backlog')
		on conflict do nothing`,
		projectID, is.Number, is.ID, is.Title, labels, is.Body, Labels(is))
	return err
}
