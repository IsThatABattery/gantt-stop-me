package automation

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/projects"
)

// Run polls every automated project's pull requests every 30 seconds until ctx ends. It does nothing while
// RunnerURL is empty. A paused project is still polled, since pausing stops only new workflow executions, unless
// GitHub refused its owner's credential.
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
// the last read: it reads the closing references of the new, changed and ready ones in one GraphQL query, and
// deletes the records of those no longer listed. An unchanged list costs one request. It does nothing while
// automation is off, its owner has no GitHub account linked, or it is paused because GitHub refused the owner's
// credential, and it pauses the project, returning ErrPaused, when GitHub refuses that credential.
func (a *Automation) Poll(ctx context.Context, projectID int64) error {
	p, token, etag, enabled, err := a.target(ctx, projectID)
	if errors.Is(err, ErrOff) || errors.Is(err, github.ErrNotLinked) || errors.Is(err, ErrPaused) {
		return nil // nothing to read GitHub with, or no reason to; OwnerToken pauses an unlinked owner's project before anything runs
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
		// Every ready one too: an edited description changes the issues it closes without a push, and the count of
		// ready pull requests closing an issue reads them.
		if r := before[pr.Number]; r == nil || !pr.Draft || r.head != pr.Head.SHA || r.draft != pr.Draft || r.author != pr.User.ID {
			views = append(views, view{pr: pr})
			ids = append(ids, pr.NodeID)
		}
	}
	if len(ids) > 0 {
		closes, err := a.GitHub.ClosingIssues(ctx, token, p.RepoID, ids)
		if err != nil {
			return a.PauseIfRejected(ctx, projectID, token, err)
		}
		kept := views[:0]
		for _, v := range views {
			v.links = closes[v.pr.Number]
			slices.Sort(v.links)
			if changed(before[v.pr.Number], v) {
				kept = append(kept, v)
			}
		}
		views = kept
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
	// Not onto automation turned off and on again meanwhile: its records were wiped, and this list never saw them go.
	_, err = a.DB.Exec(ctx, `update project_automation set pulls_etag = nullif($2, '') where project_id = $1 and enabled_at = $3`,
		projectID, newETag, enabled)
	return err
}

// changed reports whether view v of a listed pull request differs from its record r, nil for none.
func changed(r *record, v view) bool {
	return r == nil || r.head != v.pr.Head.SHA || r.headRepo != headRepo(v.pr) || r.author != v.pr.User.ID ||
		r.draft != v.pr.Draft || !slices.Equal(r.linked, v.links)
}

// target loads an automated project, its owner's GitHub credential, the ETag of its last pull request list and when
// automation was turned on. It returns ErrOff while automation is off, github.ErrNotLinked while the owner has no
// GitHub account linked, and ErrPaused while it is paused because GitHub refused that credential: until a takeover
// unpauses it, GitHub would only refuse it again.
func (a *Automation) target(ctx context.Context, projectID int64) (p projects.Project, token, etag string, enabled time.Time, err error) {
	var ownerID int64
	var login, paused string
	err = a.DB.QueryRow(ctx, `
		select p.id, p.github_repo_id, p.owner, p.name, pa.owner_id, coalesce(pa.pulls_etag, ''), pa.enabled_at,
			coalesce(gl.login, ''), coalesce(pa.paused_reason, '')
		from project_automation pa join projects p on p.id = pa.project_id
		left join github_links gl on gl.user_id = pa.owner_id where pa.project_id = $1`, projectID).
		Scan(&p.ID, &p.RepoID, &p.Owner, &p.Name, &ownerID, &etag, &enabled, &login, &paused)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, "", "", enabled, ErrOff
	}
	if err != nil {
		return p, "", "", enabled, err
	}
	if paused == rejectedReason(login) {
		return p, "", "", enabled, ErrPaused
	}
	token, err = a.Credentials.Token(ctx, ownerID)
	return p, token, etag, enabled, err
}
