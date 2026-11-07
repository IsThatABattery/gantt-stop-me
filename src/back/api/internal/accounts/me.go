package accounts

import (
	"net/http"

	"gantt-stop-me/api/internal/httpx"
)

type githubLink struct {
	Login string `json:"login"`
	Kind  string `json:"kind"`
}

// me returns the signed-in user and their linked GitHub login, or null for github.
func (a *Accounts) me(w http.ResponseWriter, r *http.Request) {
	var out struct {
		user
		GitHub *githubLink `json:"github"`
	}
	var login, kind *string
	err := a.db.QueryRow(r.Context(), `
		select u.id, u.email, g.login, g.kind
		from users u left join github_links g on g.user_id = u.id
		where u.id = $1`, UserID(r.Context())).Scan(&out.ID, &out.Email, &login, &kind)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if login != nil {
		out.GitHub = &githubLink{Login: *login, Kind: *kind}
	}
	httpx.JSON(w, http.StatusOK, out)
}
