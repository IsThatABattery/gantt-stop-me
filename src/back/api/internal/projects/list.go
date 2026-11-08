package projects

import (
	"errors"
	"log"
	"net/http"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
)

// list answers GET /api/projects: the projects the user has added or opened, most recently visited first, keeping
// only those their GitHub account still sees. It never lists repositories the user has not opened here.
func (p *Projects) list(w http.ResponseWriter, r *http.Request) {
	if out, ok := p.Opened(w, r); ok {
		httpx.JSON(w, http.StatusOK, map[string]any{"projects": out})
	}
}

// Opened returns the projects the user has added or opened and their GitHub account still sees, most recently
// visited first, any GitHub fails to check with its last checked permission, or answers the request itself and
// returns false.
//
// shortcut: each project whose cached permission is over 5 minutes old is checked on GitHub one after another, so
// dozens of stale projects make a slow answer; check them in parallel when users open that many.
func (p *Projects) Opened(w http.ResponseWriter, r *http.Request) ([]Project, bool) {
	ctx := r.Context()
	userID := accounts.UserID(ctx)
	token, ok := p.token(w, r)
	if !ok {
		return nil, false
	}
	rows, err := p.DB.Query(ctx, `
		select p.id, p.github_repo_id, p.owner, p.name, m.permission from projects p
		join project_members m on m.project_id = p.id and m.user_id = $1
		where p.synced_at is not null
		order by m.visited_at desc, p.id`, userID)
	if err != nil {
		httpx.Internal(w, r, err)
		return nil, false
	}
	var opened []Project
	for rows.Next() {
		var pr Project
		if err := rows.Scan(&pr.ID, &pr.RepoID, &pr.Owner, &pr.Name, &pr.Permission); err != nil {
			rows.Close()
			httpx.Internal(w, r, err)
			return nil, false
		}
		opened = append(opened, pr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return nil, false
	}
	out := []Project{}
	for _, pr := range opened {
		stored := pr.Permission
		pr.Permission, err = p.Permission(ctx, &pr, userID, token)
		if errors.As(err, new(gitHubError)) {
			if ghErr := (*github.Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusUnauthorized {
				github.Fail(w, r, err)
				return nil, false
			}
			// GitHub cannot check it now, such as on a 5xx: list it as last checked, since every project route checks
			// again before answering.
			log.Printf("%s %s: check project %d: %v", r.Method, r.URL.Path, pr.ID, err)
			pr.Permission, err = stored, nil
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return nil, false
		}
		if pr.Permission != "" { // lost access: Permission forgot the membership too
			out = append(out, pr)
		}
	}
	return out, true
}
