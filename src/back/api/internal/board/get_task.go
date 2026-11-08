package board

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/tasktests"
)

// getTask returns a task with its issue's cached body and labels, the card's tests and its linked pull requests,
// after a sync with GitHub if one is due.
func (b *Board) getTask(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Read)
	if !ok || !b.syncIfDue(w, r, p, token) {
		return
	}
	b.ServeTask(w, r, p, token, taskID(r))
}

// member is a user as every member may see them: by GitHub login, never by email.
type member struct {
	ID    int64   `json:"id"`
	Login *string `json:"login"` // nil once they unlink GitHub
}

// ServeTask answers task id of project p as Get task does, reading its linked pull requests from GitHub with token
// first when its last read is over 30 seconds old, else, or when that read fails, as cached. The caller has checked
// access.
func (b *Board) ServeTask(w http.ResponseWriter, r *http.Request, p projects.Project, token string, id int64) {
	ctx := r.Context()
	var pullsErr *string
	if err := b.refreshPulls(ctx, p, token, id); errors.As(err, new(GitHubError)) {
		// The cached list answers: the caller's write, when there was one, has gone through.
		log.Printf("read the pull requests of task %d: %v", id, err)
		why := "GitHub did not list the pull requests closing the issue, so they are as last read."
		pullsErr = &why
	} else if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	var out struct {
		task
		Body   string `json:"body"`
		Labels []struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"labels"` // not github.Label, whose id only the sync needs
		Tests             []tasktests.Test    `json:"tests"`
		TestsError        *string             `json:"testsError"`
		PullRequests      []github.LinkedPull `json:"pullRequests"`
		PullRequestsError *string             `json:"pullRequestsError"`
		TakenOverBy       *member             `json:"takenOverBy"` // nil while automation has it
	}
	var takerID *int64
	var takerLogin *string
	var err error
	out.task, err = b.scanTask(b.DB.QueryRow(ctx, `
		select `+taskColumns+`, t.body, t.labels, t.tests, t.tests_error, t.pull_requests, t.taken_over_by,
			(select login from github_links where user_id = t.taken_over_by)
		from tasks t where t.id = $1 and t.project_id = $2`,
		id, p.ID), p, &out.Body, &out.Labels, &out.Tests, &out.TestsError, &out.PullRequests, &takerID, &takerLogin)
	if errors.Is(err, pgx.ErrNoRows) {
		taskNotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	out.PullRequestsError = pullsErr
	if takerID != nil {
		out.TakenOverBy = &member{*takerID, takerLogin}
	}
	httpx.JSON(w, http.StatusOK, out)
}

// refreshPulls reads, with token, the pull requests that close task id's issue into its cache, at most once every
// 30 seconds per task, as the sync is throttled. A failed GitHub call comes back as a GitHubError, and the next view
// tries again.
func (b *Board) refreshPulls(ctx context.Context, p projects.Project, token string, id int64) error {
	var prev *time.Time
	err := b.DB.QueryRow(ctx, `
		update tasks set pull_requests_at = now()
		where id = $1 and project_id = $2 and (pull_requests_at is null or pull_requests_at < now() - interval '30 seconds')
		returning old.pull_requests_at`, id, p.ID).Scan(&prev)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // read in the last 30 seconds, or no such task, which the caller's read finds
	}
	if err != nil {
		return err
	}
	if _, err := b.LinkedPulls(ctx, p, token, id); err != nil {
		// Give the claim back, so the next view reads them.
		b.DB.Exec(context.WithoutCancel(ctx), `update tasks set pull_requests_at = $2 where id = $1`, id, prev)
		return err
	}
	return nil
}

// LinkedPulls reads, with token, the pull requests of project p that close task id's issue, caches them for Get task
// and returns them. The issue's GraphQL id comes from the sync, or from GitHub the first time. A failed GitHub call
// comes back as a GitHubError.
func (b *Board) LinkedPulls(ctx context.Context, p projects.Project, token string, id int64) ([]github.LinkedPull, error) {
	var number int
	var node string
	if err := b.DB.QueryRow(ctx, `select issue_number, coalesce(issue_node_id, '') from tasks where id = $1 and project_id = $2`, id, p.ID).
		Scan(&number, &node); err != nil {
		return nil, err
	}
	if node == "" {
		is, err := b.GitHub.Issue(ctx, token, p.RepoID, number)
		if err != nil {
			return nil, GitHubError{err}
		}
		node = is.NodeID
	}
	pulls, err := b.GitHub.LinkedPulls(ctx, token, p.RepoID, node)
	if err != nil {
		return nil, GitHubError{err}
	}
	_, err = b.DB.Exec(ctx, `update tasks set issue_node_id = $2, pull_requests = $3, pull_requests_at = now() where id = $1`, id, node, pulls)
	return pulls, err
}
