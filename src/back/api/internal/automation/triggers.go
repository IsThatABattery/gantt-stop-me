package automation

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/projects"
)

// ambiguous is the runner error of a test run started while more than one open, ready pull request closes its issue.
const ambiguous = "More than one open, ready pull request is linked to this issue."

// record is an open pull request as the triggers last applied it: its row in pull_requests.
type record struct {
	head     string
	headRepo int64 // 0 once the fork is deleted
	author   int64 // GitHub user id
	draft    bool
	linked   []int // numbers of the issues it closes, sorted
	refused  bool  // the "not tested" comment was posted
}

// recordColumns selects, from pull_requests, what a record holds, in its order.
const recordColumns = `head_sha, coalesce(head_repo_id, 0), author_id, draft, linked_issues, refused`

// same reports whether a and b, either nil for no record, are the same record.
func same(a, b *record) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.head == b.head && a.headRepo == b.headRepo && a.author == b.author && a.draft == b.draft &&
		a.refused == b.refused && slices.Equal(a.linked, b.linked)
}

// records returns the project's pull request records by number.
func records(ctx context.Context, db *pgxpool.Pool, projectID int64) (map[int]*record, error) {
	all := map[int]*record{}
	var n int
	var r record
	rows, _ := db.Query(ctx, `select number, `+recordColumns+` from pull_requests where project_id = $1`, projectID)
	_, err := pgx.ForEachRow(rows, []any{&n, &r.head, &r.headRepo, &r.author, &r.draft, &r.linked, &r.refused}, func() error {
		rec := r
		rec.linked = slices.Clone(r.linked)
		all[n] = &rec
		return nil
	})
	return all, err
}

// view is a pull request as one read of GitHub showed it, with the issues of the project's repository it closes,
// or gone once GitHub no longer has it open.
type view struct {
	pr    github.PullRequest
	links []int
	gone  bool
}

// headRepo is the id of the repository pr's branch is in, or 0 once that fork is deleted.
func headRepo(pr github.PullRequest) int64 {
	if pr.Head.Repo == nil {
		return 0
	}
	return pr.Head.Repo.ID
}

// apply brings the project's pull request records in line with views, each in its own transaction under the
// project's lock with the moves and test runs its change causes. So a change the poller and a webhook delivery both
// see triggers once. Views that can start a run go last, oldest pull request first, so a run's count of ready pull
// requests already sees the others closed, back in draft or closing other issues. before holds the records as they
// were before GitHub was read for views: a record changed since was saved from another read of GitHub, maybe a newer
// one, so apply leaves its pull request for the next poll and reports skipped. It stops at the first error, keeping
// the pull requests applied before it.
func (a *Automation) apply(ctx context.Context, p projects.Project, token string, before map[int]*record, views []view) (skipped bool, err error) {
	starts := func(v view) int {
		if r := before[v.pr.Number]; !v.gone && !v.pr.Draft && (r == nil || r.draft) {
			return 1
		}
		return 0
	}
	slices.SortFunc(views, func(a, b view) int {
		return cmp.Or(cmp.Compare(starts(a), starts(b)), cmp.Compare(a.pr.Number, b.pr.Number))
	})
	// Once GitHub has a move or comment, finish saving it even if ctx ends.
	saveCtx := context.WithoutCancel(ctx)
	for _, v := range views {
		if err := ctx.Err(); err != nil {
			return skipped, err
		}
		// Per pull request, so a long poll never holds up the project's moves and syncs throughout.
		unlock, err := projects.Serialize(ctx, p.ID)
		if err != nil {
			return skipped, err
		}
		err = pgx.BeginFunc(saveCtx, a.DB, func(tx pgx.Tx) error {
			applied, err := a.trigger(saveCtx, tx, p, token, before[v.pr.Number], v)
			skipped = skipped || !applied
			return err
		})
		unlock()
		if err != nil {
			return skipped, err
		}
	}
	return skipped, nil
}

// trigger applies view v of one pull request in tx, as the trigger table says: it deletes the record of a pull
// request gone, else saves the new record, cancels the pull request's test runs when it went back to draft, and calls
// ready when it became ready. It reports false, changing nothing, when the record is no longer was or automation
// was turned off.
func (a *Automation) trigger(ctx context.Context, tx pgx.Tx, p projects.Project, token string, was *record, v view) (bool, error) {
	if err := projects.Lock(ctx, tx, p.ID); err != nil {
		return false, err
	}
	var on bool
	if err := tx.QueryRow(ctx, `select exists (select from project_automation where project_id = $1)`, p.ID).Scan(&on); err != nil {
		return false, err
	}
	old := &record{}
	err := tx.QueryRow(ctx, `select `+recordColumns+` from pull_requests where project_id = $1 and number = $2`, p.ID, v.pr.Number).
		Scan(&old.head, &old.headRepo, &old.author, &old.draft, &old.linked, &old.refused)
	if errors.Is(err, pgx.ErrNoRows) {
		old, err = nil, nil
	}
	if err != nil || !on || !same(old, was) {
		return false, err
	}
	if v.gone {
		// shortcut: the refused mark goes with the record, so a refused pull request closed and reopened is commented
		// on again; keep refused numbers apart from the records if authors cycle them.
		_, err := tx.Exec(ctx, `delete from pull_requests where project_id = $1 and number = $2`, p.ID, v.pr.Number)
		return true, err
	}
	links := append([]int{}, v.links...) // not nil, which would store NULL
	slices.Sort(links)
	_, err = tx.Exec(ctx, `
		insert into pull_requests (project_id, number, head_sha, head_repo_id, author_id, draft, linked_issues)
		values ($1, $2, $3, nullif($4, 0), $5, $6, $7)
		on conflict (project_id, number) do update set head_sha = excluded.head_sha, head_repo_id = excluded.head_repo_id,
			author_id = excluded.author_id, draft = excluded.draft, linked_issues = excluded.linked_issues`,
		p.ID, v.pr.Number, v.pr.Head.SHA, headRepo(v.pr), v.pr.User.ID, v.pr.Draft, links)
	switch {
	case err != nil:
		return false, err
	case v.pr.Draft && old != nil && !old.draft:
		return true, cancelRuns(ctx, tx, p.ID, 0, v.pr.Number, "The pull request went back to draft.")
	case !v.pr.Draft && (old == nil || old.draft):
		return true, a.ready(ctx, tx, p, token, v.pr, links, old != nil && old.refused)
	}
	return true, nil // a new head while ready, or no change the triggers act on
}

// ready acts on pr becoming ready for review, closing the issues links: each of their tasks in an open column moves
// to the end of Testing, unless it is there already, and gets a new test run. A pull request that may not run gets
// one comment saying why instead, unless refused says it has it, and nothing else happens. A task in Closed is never
// moved and gets no run.
func (a *Automation) ready(ctx context.Context, tx pgx.Tx, p projects.Project, token string, pr github.PullRequest, links []int, refused bool) error {
	type linked struct {
		id     int64
		column string // key
		ready  int    // open, ready pull requests closing its issue, pr included, but no fork or refused one
	}
	// A fork or refused pull request never counts: it would let anyone who opens one turn a trusted run into an error.
	rows, _ := tx.Query(ctx, `
		select t.id, c.key, (select count(*) from pull_requests r
			where r.project_id = t.project_id and not r.draft and t.issue_number = any(r.linked_issues)
				and r.head_repo_id = $3 and not r.refused)
		from tasks t join columns c on c.id = t.column_id
		where t.project_id = $1 and t.issue_number = any($2) and not c.closes_issue
		order by t.issue_number`, p.ID, links, p.RepoID)
	tasks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (linked, error) {
		var l linked
		return l, row.Scan(&l.id, &l.column, &l.ready)
	})
	if err != nil || len(tasks) == 0 {
		return err
	}
	why, err := a.refusal(ctx, token, p, pr)
	if err != nil {
		return err
	}
	if why != "" {
		if refused {
			return nil
		}
		if err := a.GitHub.CreateComment(ctx, token, p.RepoID, pr.Number, why); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `update pull_requests set refused = true where project_id = $1 and number = $2`, p.ID, pr.Number)
		return err
	}
	for _, t := range tasks {
		if t.column != "testing" {
			if err := a.Board.MoveToEnd(ctx, tx, token, p, t.id, "testing"); err != nil {
				return err
			}
		}
		number, head, failure := pr.Number, pr.Head.SHA, ""
		if t.ready > 1 {
			number, head, failure = 0, "", ambiguous // no one pull request to test
		}
		if err := startRun(ctx, tx, p.ID, t.id, number, head, failure); err != nil {
			return err
		}
	}
	return nil
}

// refusal says why pr may not run, as the comment that tells its author, or is "" when it may: it must come from a
// branch of the project's repository, by an author with write access or above.
func (a *Automation) refusal(ctx context.Context, token string, p projects.Project, pr github.PullRequest) (string, error) {
	if headRepo(pr) != p.RepoID {
		return "Not tested: this pull request comes from a fork. Automation tests only pull requests from branches of this repository.", nil
	}
	if ok, err := a.canPush(ctx, token, p, pr.User.Login); err != nil || ok {
		return "", err
	}
	return "Not tested: " + pr.User.Login + " does not have write access to this repository. " +
		"Automation tests only pull requests by authors who can push to it.", nil
}

// canPush reports whether login has write access or above to the project's repository, as GitHub tells token.
func (a *Automation) canPush(ctx context.Context, token string, p projects.Project, login string) (bool, error) {
	if login == "" { // no author to ask about, which would ask for the collaborators list instead
		return false, nil
	}
	perm, err := a.GitHub.Permission(ctx, token, p.RepoID, login)
	return perm == "admin" || perm == "maintain" || perm == "write", err
}

// startRun starts a test run of task taskID on head of pull request number: it cancels the task's earlier queued or
// running run, then queues the new run and clears the task's needs_attention. With failure set, the run ends at once
// as a runner error with that reason, which flags the task instead. number 0 and head "" record no pull request.
// Call it in tx holding projects.Lock.
func startRun(ctx context.Context, tx pgx.Tx, projectID, taskID int64, number int, head, failure string) error {
	if err := cancelRuns(ctx, tx, projectID, taskID, 0, "A new test run started."); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		with flag as (update tasks set needs_attention = nullif($4, '') where id = $1)
		insert into test_runs (task_id, pr_number, head_sha, status, reason, finished_at)
		values ($1, nullif($2, 0), nullif($3, ''), case when $4 = '' then 'queued' else 'error' end, nullif($4, ''),
			case when $4 <> '' then now() end)`, taskID, number, head, failure)
	return err
}

// cancelRuns cancels the project's queued and running test runs of task taskID, or when taskID is 0, those testing
// pull request number, with reason, then skips the rest of them.
func cancelRuns(ctx context.Context, tx pgx.Tx, projectID, taskID int64, number int, reason string) error {
	rows, _ := tx.Query(ctx, `
		update test_runs set status = 'cancelled', reason = $4, finished_at = now()
		where status in ('queued', 'running') and task_id in (select id from tasks where project_id = $1)
			and (task_id = $2 or $2 = 0 and pr_number = $3)
		returning id`, projectID, taskID, number, reason)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil || len(ids) == 0 {
		return err
	}
	return skipRest(ctx, tx, ids)
}

// skipRest skips the unfinished tests of test runs ids, just cancelled, and cancels their queued workflow
// executions. A running one keeps its status until the queue worker has stopped it and removed it from the runner,
// so the queue starts nothing beside it meanwhile. A statement of its own, after the runs': it sees what a plan
// committed while the cancel waited for the run's row.
func skipRest(ctx context.Context, tx pgx.Tx, ids []int64) error {
	_, err := tx.Exec(ctx, `
		with results as (update test_results set status = 'skipped' where status = 'pending' and run_id = any($1))
		update workflow_executions set status = 'cancelled', finished_at = now() where status = 'queued' and run_id = any($1)`, ids)
	return err
}
