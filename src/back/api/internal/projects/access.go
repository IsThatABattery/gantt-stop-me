package projects

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
)

// The permissions a project route can need.
const (
	Read  = "read"
	Write = "write"
)

var rank = map[string]int{"read": 1, "write": 2, "admin": 3}

// permission maps GitHub's permissions on a repository to the app's, or "" for none.
func permission(r github.Repo) string {
	switch p := r.Permissions; {
	case p.Admin:
		return "admin"
	case p.Push || p.Maintain:
		return "write"
	case p.Pull || p.Triage:
		return "read"
	}
	return ""
}

// Check runs the access check for the project in the {id} path value: the user's linked GitHub account must
// see its repository with need, Read or Write, or more. It returns the project and the user's GitHub token,
// or answers the request itself and returns false.
func (p *Projects) Check(w http.ResponseWriter, r *http.Request, need string) (Project, string, bool) {
	ctx := r.Context()
	userID := accounts.UserID(ctx)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64) // 0, which matches no project, when not a number
	var pr Project
	// A project whose initialization has not finished has no board yet.
	err := p.DB.QueryRow(ctx, `select id, github_repo_id, owner, name from projects where id = $1 and synced_at is not null`, id).
		Scan(&pr.ID, &pr.RepoID, &pr.Owner, &pr.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w)
		return Project{}, "", false
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return Project{}, "", false
	}
	token, ok := p.token(w, r)
	if !ok {
		return Project{}, "", false
	}

	err = p.DB.QueryRow(ctx, `
		select permission from project_members
		where project_id = $1 and user_id = $2 and checked_at > now() - interval '5 minutes'`, pr.ID, userID).Scan(&pr.Permission)
	if errors.Is(err, pgx.ErrNoRows) {
		var repo github.Repo
		repo, err = p.GitHub.RepoByID(ctx, token, pr.RepoID)
		if err == nil {
			pr.Permission = permission(repo)
		}
		switch {
		case hidden(err) || err == nil && pr.Permission == "":
			if _, err := p.DB.Exec(ctx, `delete from project_members where project_id = $1 and user_id = $2`, pr.ID, userID); err != nil {
				httpx.Internal(w, r, err)
				return Project{}, "", false
			}
			notFound(w)
			return Project{}, "", false
		case err != nil:
			github.Fail(w, r, err)
			return Project{}, "", false
		}
		if repo.Owner.Login != pr.Owner || repo.Name != pr.Name {
			// Renamed or transferred on GitHub: show the name GitHub has now, which task links use.
			pr.Owner, pr.Name = repo.Owner.Login, repo.Name
			if _, err := p.DB.Exec(ctx, `update projects set owner = $2, name = $3 where id = $1`, pr.ID, pr.Owner, pr.Name); err != nil {
				httpx.Internal(w, r, err)
				return Project{}, "", false
			}
		}
		err = p.remember(ctx, pr.ID, userID, pr.Permission)
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return Project{}, "", false
	}
	if rank[pr.Permission] < rank[need] {
		httpx.Error(w, http.StatusForbidden, "forbidden", "This needs "+need+" access to the repository on GitHub.")
		return Project{}, "", false
	}
	return pr, token, true
}

// token returns the user's GitHub credential, or answers 403 github_not_linked and returns false.
func (p *Projects) token(w http.ResponseWriter, r *http.Request) (string, bool) {
	token, err := github.Token(r.Context(), p.DB, p.Secrets, accounts.UserID(r.Context()))
	if errors.Is(err, github.ErrNotLinked) {
		httpx.Error(w, http.StatusForbidden, "github_not_linked", "Link your GitHub account in Settings first.")
		return "", false
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return "", false
	}
	return token, true
}

// remember caches the user's permission on the project as checked now.
func (p *Projects) remember(ctx context.Context, projectID, userID int64, permission string) error {
	_, err := p.DB.Exec(ctx, `
		insert into project_members (project_id, user_id, permission, checked_at) values ($1, $2, $3, now())
		on conflict (project_id, user_id) do update set permission = excluded.permission, checked_at = excluded.checked_at`,
		projectID, userID, permission)
	return err
}

// hidden reports whether GitHub answered that the user cannot see the repository.
func hidden(err error) bool {
	var ghErr *github.Error
	return errors.As(err, &ghErr) && (ghErr.Status == http.StatusNotFound || ghErr.Status == http.StatusForbidden)
}

// notFound is the one answer for a missing project and a repository the user cannot see,
// so a response never reveals that a private repository exists.
func notFound(w http.ResponseWriter) {
	httpx.Error(w, http.StatusNotFound, "not_found", "Repository not found, or your GitHub account cannot see it.")
}
