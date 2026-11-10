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
// see its repository with need, Read or Write, or more. It returns the project and the user's GitHub token, the one
// read again when another request replaced the first one read meanwhile, or answers the request itself and returns
// false.
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

	pr.Permission, token, err = p.recheck(ctx, &pr, userID, token)
	if errors.As(err, new(gitHubError)) {
		github.Fail(w, r, err)
		return Project{}, "", false
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return Project{}, "", false
	}
	if pr.Permission == "" {
		notFound(w)
		return Project{}, "", false
	}
	if !Allows(pr.Permission, need) {
		httpx.Error(w, http.StatusForbidden, "forbidden", "This needs "+need+" access to the repository on GitHub.")
		return Project{}, "", false
	}
	return pr, token, true
}

// Allows reports whether permission, as Permission returns it, meets need, Read or Write.
func Allows(permission, need string) bool { return rank[permission] >= rank[need] }

// Permission returns the user's permission on the project, "read", "write" or "admin", or "" when their GitHub
// account cannot see its repository. It answers from a check under 5 minutes old, else asks GitHub with token and
// remembers the answer, forgetting the user's access when GitHub hides the repository. A rename or transfer GitHub
// reports updates pr and the stored project. A failed GitHub call returns its error wrapped; errors.As still finds a *github.Error.
func (p *Projects) Permission(ctx context.Context, pr *Project, userID int64, token string) (string, error) {
	var perm string
	err := p.DB.QueryRow(ctx, `
		select permission from project_members
		where project_id = $1 and user_id = $2 and checked_at > now() - interval '5 minutes'`, pr.ID, userID).Scan(&perm)
	if !errors.Is(err, pgx.ErrNoRows) {
		return perm, err
	}
	repo, err := p.GitHub.RepoByID(ctx, token, pr.RepoID)
	if err == nil {
		perm = permission(repo)
	}
	switch {
	case hidden(err) || err == nil && perm == "":
		_, err := p.DB.Exec(ctx, `delete from project_members where project_id = $1 and user_id = $2`, pr.ID, userID)
		return "", err
	case err != nil:
		return "", gitHubError{err}
	}
	if repo.Owner.Login != pr.Owner || repo.Name != pr.Name {
		// Renamed or transferred on GitHub: show the name GitHub has now, which task links use.
		pr.Owner, pr.Name = repo.Owner.Login, repo.Name
		if _, err := p.DB.Exec(ctx, `update projects set owner = $2, name = $3 where id = $1`, pr.ID, pr.Owner, pr.Name); err != nil {
			return "", err
		}
	}
	return perm, p.remember(ctx, pr.ID, userID, perm)
}

// recheck is Permission with token, asked again with the user's credential read anew when GitHub refused token with
// 401 and the credential has since become another token: GitHub retires a token as another request refreshes it. It
// returns the token the answer came with.
func (p *Projects) recheck(ctx context.Context, pr *Project, userID int64, token string) (perm, used string, err error) {
	perm, err = p.Permission(ctx, pr, userID, token)
	if !unauthorized(err) {
		return perm, token, err
	}
	current, tErr := p.Credentials.Token(ctx, userID)
	if tErr != nil || current == token {
		return perm, token, err // GitHub refused the credential in use now, or it cannot be read to tell
	}
	perm, err = p.Permission(ctx, pr, userID, current)
	return perm, current, err
}

// Visited records that the user opened the project now, which orders Opened.
func (p *Projects) Visited(ctx context.Context, projectID, userID int64) error {
	_, err := p.DB.Exec(ctx, `update project_members set visited_at = now() where project_id = $1 and user_id = $2`, projectID, userID)
	return err
}

// gitHubError marks Permission's failed GitHub call apart from a database error, so Check answers it with github.Fail.
type gitHubError struct{ error }

func (e gitHubError) Unwrap() error { return e.error }

// token returns the user's GitHub credential, or answers 403 github_not_linked, or 502 github_error for an expired
// OAuth token GitHub could not refresh, and returns false.
func (p *Projects) token(w http.ResponseWriter, r *http.Request) (string, bool) {
	token, err := p.Credentials.Token(r.Context(), accounts.UserID(r.Context()))
	if errors.Is(err, github.ErrNotLinked) {
		httpx.Error(w, http.StatusForbidden, "github_not_linked", "Link your GitHub account in Settings first.")
		return "", false
	}
	if errors.Is(err, github.ErrUnavailable) {
		github.Fail(w, r, err)
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

// unauthorized reports whether err is GitHub refusing the credential itself with 401.
func unauthorized(err error) bool {
	var ghErr *github.Error
	return errors.As(err, &ghErr) && ghErr.Status == http.StatusUnauthorized
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
