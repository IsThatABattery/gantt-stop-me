package automation

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/projects"
)

// Run polls every automated project's pull requests every 30 seconds until ctx ends. It does nothing while
// RunnerURL is empty. A paused project is still polled: pausing stops only new workflow executions.
// shortcut: projects are polled one after another, so many of them or a slow GitHub stretch the 30 seconds; poll
// them concurrently when that bites.
func (a *Automation) Run(ctx context.Context) {
	if a.RunnerURL == "" {
		return
	}
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		rows, _ := a.DB.Query(ctx, `select project_id from project_automation order by project_id`)
		ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
		if err != nil && ctx.Err() == nil {
			log.Printf("poll: %v", err)
		}
		for _, id := range ids {
			pollCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			if err := a.Poll(pollCtx, id); err != nil && ctx.Err() == nil {
				log.Printf("poll project %d: %v", id, err)
			}
			cancel()
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Poll reads the project's open pull requests once, as Run does every 30 seconds, and applies what changed since
// the last read: it reads the closing references of the new and changed ones in one GraphQL query, and deletes the
// records of those no longer listed. An unchanged list costs one request. It does nothing while automation is off or
// its owner has no GitHub account linked, and it pauses the project, returning ErrPaused, when GitHub refuses the
// owner's credential.
func (a *Automation) Poll(ctx context.Context, projectID int64) error {
	p, token, etag, err := a.target(ctx, projectID)
	if errors.Is(err, ErrOff) || errors.Is(err, github.ErrNotLinked) {
		return nil // OwnerToken pauses an unlinked owner's project before anything runs
	}
	if err != nil {
		return err
	}
	// Read before GitHub is, so apply can tell a record that another read of GitHub saved meanwhile.
	before, err := records(ctx, a.DB, projectID)
	if err != nil {
		return err
	}
	pulls, newETag, err := a.GitHub.PullRequests(ctx, token, p.RepoID, etag)
	if ghErr := (*github.Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusNotModified {
		return nil
	}
	if err != nil {
		return a.PauseIfRejected(ctx, projectID, token, err)
	}
	var views []view
	var ids []string
	listed := map[int]bool{}
	for _, pr := range pulls {
		listed[pr.Number] = true
		if r := before[pr.Number]; r == nil || r.head != pr.Head.SHA || r.draft != pr.Draft || r.author != pr.User.ID {
			views = append(views, view{pr: pr})
			ids = append(ids, pr.NodeID)
		}
	}
	if len(ids) > 0 {
		closes, err := a.GitHub.ClosingIssues(ctx, token, p.RepoID, ids)
		if err != nil {
			return a.PauseIfRejected(ctx, projectID, token, err)
		}
		for i := range views {
			views[i].links = closes[views[i].pr.Number]
		}
	}
	for n := range before {
		if !listed[n] {
			views = append(views, view{pr: github.PullRequest{Number: n}, gone: true})
		}
	}
	skipped, err := a.apply(ctx, p, token, before, views)
	if err != nil {
		return a.PauseIfRejected(ctx, projectID, token, err)
	}
	if skipped {
		return nil // keep the old ETag, so the next poll lists them again
	}
	_, err = a.DB.Exec(ctx, `update project_automation set pulls_etag = nullif($2, '') where project_id = $1`, projectID, newETag)
	return err
}

// target loads an automated project, its owner's GitHub credential and the ETag of its last pull request list.
// It returns ErrOff while automation is off, and github.ErrNotLinked while the owner has no GitHub account linked.
func (a *Automation) target(ctx context.Context, projectID int64) (p projects.Project, token, etag string, err error) {
	var ownerID int64
	err = a.DB.QueryRow(ctx, `
		select p.id, p.github_repo_id, p.owner, p.name, pa.owner_id, coalesce(pa.pulls_etag, '')
		from project_automation pa join projects p on p.id = pa.project_id where pa.project_id = $1`, projectID).
		Scan(&p.ID, &p.RepoID, &p.Owner, &p.Name, &ownerID, &etag)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, "", "", ErrOff
	}
	if err != nil {
		return p, "", "", err
	}
	token, err = github.Token(ctx, a.DB, a.Secrets, ownerID)
	return p, token, etag, err
}
