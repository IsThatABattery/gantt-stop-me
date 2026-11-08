package automation

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/projects"
)

var (
	// ErrPaused means the project's automation is paused, so no workflow execution may start. Only a takeover unpauses it.
	ErrPaused = errors.New("automation: paused")
	// ErrOff means the project's automation is off.
	ErrOff = errors.New("automation: off")
)

// OwnerToken returns the automation owner's GitHub credential, for a workflow execution about to start. It first
// checks they can still push to the repository, from the same 5-minute cache as project routes, and pauses the
// project when they cannot, have unlinked GitHub, or GitHub no longer accepts their credential. It returns ErrPaused
// then and while already paused, and ErrOff while automation is off. Any other error, such as GitHub's 429 or 5xx,
// is temporary and pauses nothing.
func (a *Automation) OwnerToken(ctx context.Context, projectID int64) (string, error) {
	var pr projects.Project
	var ownerID int64
	var login string
	var paused bool
	err := a.DB.QueryRow(ctx, `
		select p.id, p.github_repo_id, p.owner, p.name, pa.owner_id, coalesce(gl.login, ''), pa.paused_reason is not null
		from project_automation pa join projects p on p.id = pa.project_id
		left join github_links gl on gl.user_id = pa.owner_id
		where pa.project_id = $1`, projectID).Scan(&pr.ID, &pr.RepoID, &pr.Owner, &pr.Name, &ownerID, &login, &paused)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrOff
	}
	if err != nil {
		return "", err
	}
	if paused {
		return "", ErrPaused
	}
	token, err := github.Token(ctx, a.DB, a.Secrets, ownerID)
	if errors.Is(err, github.ErrNotLinked) {
		return "", a.pause(ctx, projectID, ownerID, "The automation owner unlinked their GitHub account.")
	}
	if err != nil {
		return "", err
	}
	perm, err := a.Projects.Permission(ctx, &pr, ownerID, token)
	if rejected(err) {
		return "", a.pause(ctx, projectID, ownerID, rejectedReason(login))
	}
	if err != nil {
		return "", err
	}
	if !projects.Allows(perm, projects.Write) {
		return "", a.pause(ctx, projectID, ownerID, login+" lost write access to the repository.")
	}
	return token, nil
}

// PauseIfRejected pauses the project when err is GitHub refusing token, the credential OwnerToken returned, with 401,
// and then returns ErrPaused; any other err comes back as it is. Pass it the error of every GitHub call made with
// the owner's credential. A credential the owner has since replaced, or one from an owner since taken over from,
// pauses nothing.
func (a *Automation) PauseIfRejected(ctx context.Context, projectID int64, token string, err error) error {
	if !rejected(err) {
		return err
	}
	var ownerID int64
	var login string
	if qErr := a.DB.QueryRow(ctx, `
		select pa.owner_id, coalesce(gl.login, '') from project_automation pa
		left join github_links gl on gl.user_id = pa.owner_id where pa.project_id = $1`, projectID).Scan(&ownerID, &login); qErr != nil {
		return err // off, or the database failed: the next OwnerToken judges the credential
	}
	if current, tErr := github.Token(ctx, a.DB, a.Secrets, ownerID); tErr != nil || current != token {
		return err // unlinked, relinked or a new owner: the next OwnerToken judges the credential in use now
	}
	return a.pause(ctx, projectID, ownerID, rejectedReason(login))
}

// pause records why the project's automation stopped, unless it is paused already or ownerID is no longer its owner,
// and returns ErrPaused.
func (a *Automation) pause(ctx context.Context, projectID, ownerID int64, reason string) error {
	if _, err := a.DB.Exec(ctx, `
		update project_automation set paused_reason = $3
		where project_id = $1 and owner_id = $2 and paused_reason is null`, projectID, ownerID, reason); err != nil {
		return err
	}
	return ErrPaused
}

// rejected reports whether err is GitHub refusing the credential itself, which only a new credential mends.
func rejected(err error) bool {
	var ghErr *github.Error
	return errors.As(err, &ghErr) && ghErr.Status == http.StatusUnauthorized
}

// rejectedReason is the paused_reason for GitHub refusing login's credential.
func rejectedReason(login string) string {
	return "GitHub no longer accepts " + login + "'s credential."
}
