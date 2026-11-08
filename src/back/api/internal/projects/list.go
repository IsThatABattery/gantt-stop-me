package projects

import (
	"errors"
	"net/http"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
)

// list answers GET /api/projects: the projects the user has added or opened, most recently checked first, keeping
// only those their GitHub account still sees. It never lists repositories the user has not opened here.
//
// shortcut: each project whose cached permission is over 5 minutes old is checked on GitHub one after another, so
// dozens of stale projects make a slow answer; check them in parallel when users open that many.
func (p *Projects) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := accounts.UserID(ctx)
	token, ok := p.token(w, r)
	if !ok {
		return
	}
	// checked_at moves at most every 5 minutes as the user opens the project, so it orders by recent use.
	rows, err := p.DB.Query(ctx, `
		select p.id, p.github_repo_id, p.owner, p.name from projects p
		join project_members m on m.project_id = p.id and m.user_id = $1
		where p.synced_at is not null
		order by m.checked_at desc, p.id`, userID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	var opened []Project
	for rows.Next() {
		var pr Project
		if err := rows.Scan(&pr.ID, &pr.RepoID, &pr.Owner, &pr.Name); err != nil {
			rows.Close()
			httpx.Internal(w, r, err)
			return
		}
		opened = append(opened, pr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	out := []Project{}
	for _, pr := range opened {
		pr.Permission, err = p.Permission(ctx, &pr, userID, token)
		if errors.As(err, new(gitHubError)) {
			github.Fail(w, r, err)
			return
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		if pr.Permission != "" { // lost access: Permission forgot the membership too
			out = append(out, pr)
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"projects": out})
}
