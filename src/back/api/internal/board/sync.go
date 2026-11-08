package board

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/projects"
)

// lookback re-reads issues updated a little before the last sync, since GitHub's clock is not ours
// and its issue list can lag behind an update.
const lookback = 2 * time.Minute

// syncIfDue runs sync for a route that serves the project's cached tasks. Only when GitHub no longer accepts the
// credential does it answer the request itself and return false.
func (b *Board) syncIfDue(w http.ResponseWriter, r *http.Request, p projects.Project, token string) bool {
	// Not cancelled with the request: a long read, such as a backfill a proxy times out, still finishes and saves its work.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Minute)
	defer cancel()
	if err := b.sync(ctx, p, token); err != nil {
		if ghErr := (*github.Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusUnauthorized {
			github.Fail(w, r, err) // a dead credential sends the user to Settings, as on every other project route
			return false
		}
		// The cached tasks are still worth showing, and the next load retries.
		log.Printf("sync project %d: %v", p.ID, err)
	}
	return true
}

// sync refreshes the project's tasks from its labels and issues on GitHub, at most once every 30 seconds per
// project, with the token of the user loading the board. While the project's backfill is pending, every sync reads
// every issue, and only one that applies them all clears it.
// shortcut: syncs that claim during a long backfill each read every issue, costing rate limit on big repos;
// hold the claim for the whole read if that bites.
func (b *Board) sync(ctx context.Context, p projects.Project, token string) error {
	var prev *time.Time
	var etag, labelsETag string
	var claimed time.Time
	var full bool
	err := b.DB.QueryRow(ctx, `
		update projects set synced_at = now()
		where id = $1 and (synced_at is null or synced_at < now() - interval '30 seconds')
		returning old.synced_at, coalesce(issues_etag, ''), coalesce(labels_etag, ''), new.synced_at, backfill_pending`,
		p.ID).Scan(&prev, &etag, &labelsETag, &claimed, &full)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // synced in the last 30 seconds, or another load is syncing now
	}
	if err != nil {
		return err
	}

	var cutoff time.Time
	if full {
		etag = "" // an ETag another sync stored meanwhile must not stop the full read
	} else if prev != nil {
		cutoff = prev.Add(-lookback)
	}
	var issues []github.Issue
	var newETag string
	err = b.syncLabels(ctx, p, token, labelsETag)
	if err == nil {
		query := url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {"100"}}
		newETag, err = b.GitHub.Issues(ctx, token, p.RepoID, query, etag, func(page []github.Issue) bool {
			issues = append(issues, page...)
			return len(page) > 0 && !page[len(page)-1].UpdatedAt.Before(cutoff)
		})
	}
	if ghErr := (*github.Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusNotModified {
		return nil
	}
	skipped := false
	if err == nil {
		skipped, err = b.apply(ctx, p.ID, issues, claimed)
	}
	if err != nil {
		// Give the claim back, so the next load retries instead of skipping these changes.
		b.DB.Exec(context.WithoutCancel(ctx), `update projects set synced_at = $2 where id = $1 and synced_at = $3`, p.ID, prev, claimed)
		return err
	}
	if skipped {
		// Give the claim back and drop the ETag, so the next sync refetches from the old cutoff,
		// which reaches the skipped tasks' issues even when their changes are older than this sync.
		_, err = b.DB.Exec(context.WithoutCancel(ctx), `update projects set synced_at = $2, issues_etag = null where id = $1 and synced_at = $3`, p.ID, prev, claimed)
		return err
	}
	_, err = b.DB.Exec(ctx, `update projects set issues_etag = nullif($2, ''), backfill_pending = backfill_pending and not $3 where id = $1`,
		p.ID, newETag, full)
	return err
}

// syncLabels rewrites each label stored on the project's tasks to the repository's label with its id, keeping their
// order and dropping those the repository no longer has, unless the label list is unchanged since etag. A label
// stored with no id stays as it is until the backfill re-reads its issue. It leaves version and updated_at alone:
// a label edit moves no task, and updated_at tells a sync whether a task changed since its fetch.
func (b *Board) syncLabels(ctx context.Context, p projects.Project, token, etag string) error {
	labels, etag, err := b.GitHub.Labels(ctx, token, p.RepoID, etag)
	if ghErr := (*github.Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusNotModified {
		return nil
	}
	if err != nil {
		return err
	}
	// The <@ test leaves unwritten a task whose labels all still match the repository's.
	_, err = b.DB.Exec(ctx, `
		update tasks t set labels = coalesce((
			select jsonb_agg(coalesce(r.label, l.label) order by l.n)
			from jsonb_array_elements(t.labels) with ordinality l(label, n)
			left join jsonb_array_elements($2::jsonb) r(label) on r.label->'id' = l.label->'id'
			where r.label is not null or l.label->'id' is null
		), '[]')
		where t.project_id = $1 and not t.labels <@ $2::jsonb`, p.ID, labels)
	if err == nil {
		_, err = b.DB.Exec(ctx, `update projects set labels_etag = nullif($2, '') where id = $1`, p.ID, etag)
	}
	return err
}

// apply brings the project's tasks in line with issues, as GitHub listed them after claimed.
// It reports whether it skipped a task moved since then.
func (b *Board) apply(ctx context.Context, projectID int64, issues []github.Issue, claimed time.Time) (skipped bool, err error) {
	unlock, err := projects.Serialize(ctx, projectID)
	if err != nil {
		return false, err
	}
	defer unlock()
	err = pgx.BeginFunc(ctx, b.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, projectID); err != nil {
			return err
		}
		cols, err := columns(ctx, tx, projectID)
		if err != nil {
			return err
		}
		// Oldest change first, so new tasks are appended in the order their issues changed.
		for i := len(issues) - 1; i >= 0; i-- {
			is := issues[i]
			if is.PullRequest != nil {
				continue
			}
			var id int64
			var movedSince bool
			var seen []string
			var cur column
			// for update: an edit that lands after this select waits for the refresh, so the refresh cannot undo it.
			err := tx.QueryRow(ctx, `
				select t.id, t.updated_at > $3, t.board_labels, c.id, c.key, c.name, c.position, c.closes_issue
				from tasks t join columns c on c.id = t.column_id
				where t.project_id = $1 and t.github_issue_id = $2 for update of t`, projectID, is.ID, claimed).
				Scan(&id, &movedSince, &seen, &cur.ID, &cur.Key, &cur.Name, &cur.Position, &cur.ClosesIssue)
			if errors.Is(err, pgx.ErrNoRows) {
				if is.State == "open" {
					err = projects.AddTask(ctx, tx, projectID, is)
				} else {
					err = nil // a closed issue the board never had stays off it
				}
			} else if err == nil && movedSince {
				skipped = true // a move or edit landed after the fetch began; its issue may be stale here
			} else if err == nil {
				err = refresh(ctx, tx, id, cur, target(cols, cur, is, seen), is)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	return skipped, err
}

// target is the column a task in cur belongs in, given its issue on GitHub and the board labels seen on it before:
// Closed for a closed issue; for an open one the first column in board order whose label was added since,
// other than cur and Closed, else Backlog when it is in Closed, else cur. Only an added label moves the task,
// so one left on the issue, such as by a close on GitHub, cannot pull it back on every sync.
func target(cols []column, cur column, is github.Issue, seen []string) column {
	labels := projects.BoardLabels(is)
	open := is.State == "open"
	backlog := cur
	for _, c := range cols {
		switch {
		case !open && c.ClosesIssue && !cur.ClosesIssue:
			return c
		case open && c.ID != cur.ID && !c.ClosesIssue && slices.Contains(labels, c.label()) && !slices.Contains(seen, c.label()):
			return c
		case c.Key == "backlog":
			backlog = c
		}
	}
	if open && cur.ClosesIssue {
		return backlog
	}
	return cur
}

// refresh overwrites the task's cached title, state, body, labels and board labels, and moves it to the end of
// column to when that is not cur.
func refresh(ctx context.Context, tx pgx.Tx, id int64, cur, to column, is github.Issue) error {
	if to.ID != cur.ID {
		_, err := tx.Exec(ctx, `
			update tasks set column_id = $2, position = coalesce((select max(position) from tasks where column_id = $2), 0) + 1,
				version = version + 1, updated_at = now()
			where id = $1`, id, to.ID)
		if err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `update tasks set title = $2, state = $3, board_labels = $4, body = $5, labels = $6 where id = $1`,
		id, is.Title, is.State, projects.BoardLabels(is), is.Body, projects.Labels(is))
	return err
}
