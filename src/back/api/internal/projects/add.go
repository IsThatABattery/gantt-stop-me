package projects

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// repoURL matches https://github.com/{owner}/{name}, with an optional .git and one optional trailing slash.
var repoURL = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9-]{1,39})/([A-Za-z0-9._-]{1,100}?)(?:\.git)?/?$`)

// add adds the repository at a URL as a project, initializing it the first time, or returns the existing project.
func (p *Projects) add(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL string `json:"url"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	m := repoURL.FindStringSubmatch(in.URL)
	if m == nil || m[2] == "." || m[2] == ".." {
		httpx.Error(w, http.StatusBadRequest, "invalid_repo_url", "Enter a repository URL like https://github.com/owner/name.")
		return
	}
	token, ok := p.token(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	userID := accounts.UserID(ctx)
	repo, err := p.GitHub.Repo(ctx, token, m[1], m[2])
	if hidden(err) || err == nil && permission(repo) == "" {
		notFound(w)
		return
	}
	if err != nil {
		p.Credentials.Fail(w, r, userID, token, err)
		return
	}

	pr := Project{RepoID: repo.ID, Permission: permission(repo)}
	if rank[pr.Permission] < rank[Write] {
		// Initializing creates labels, which needs write access, so read access can only open a finished project.
		err := p.DB.QueryRow(ctx, `select id, owner, name from projects where github_repo_id = $1 and synced_at is not null`, repo.ID).
			Scan(&pr.ID, &pr.Owner, &pr.Name)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusForbidden, "forbidden", "Adding a repository needs write access to it on GitHub.")
			return
		}
		if err == nil {
			err = p.remember(ctx, pr.ID, userID, pr.Permission)
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, pr)
		return
	}

	var created, ready bool
	err = p.DB.QueryRow(ctx, `
		insert into projects (github_repo_id, owner, name, added_by) values ($1, $2, $3, $4)
		on conflict (github_repo_id) do update set owner = excluded.owner, name = excluded.name
		returning id, owner, name, old.id is null, synced_at is not null`,
		repo.ID, repo.Owner.Login, repo.Name, userID).Scan(&pr.ID, &pr.Owner, &pr.Name, &created, &ready)
	if err == nil {
		err = p.remember(ctx, pr.ID, userID, pr.Permission)
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if !ready && !p.initialize(w, r, token, pr) {
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.JSON(w, status, pr)
}
