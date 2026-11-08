package accounts

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"gantt-stop-me/api/internal/httpx"
)

type githubLink struct {
	Login string `json:"login"`
	Kind  string `json:"kind"`
}

// me returns the signed-in user, their display name or null, and their linked GitHub login, or null for github.
func (a *Accounts) me(w http.ResponseWriter, r *http.Request) {
	var out struct {
		user
		Name   *string     `json:"name"`
		GitHub *githubLink `json:"github"`
	}
	var login, kind *string
	err := a.db.QueryRow(r.Context(), `
		select u.id, u.email, u.name, g.login, g.kind
		from users u left join github_links g on g.user_id = u.id
		where u.id = $1`, UserID(r.Context())).Scan(&out.ID, &out.Email, &out.Name, &login, &kind)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if login != nil {
		out.GitHub = &githubLink{Login: *login, Kind: *kind}
	}
	httpx.JSON(w, http.StatusOK, out)
}

// editMe sets the user's display name, or clears it for null, and answers as me does.
func (a *Accounts) editMe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name *string `json:"name"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if n := utf8.RuneCountInString(name); n < 1 || n > 100 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			httpx.Error(w, http.StatusBadRequest, "invalid_name", "A name needs 1 to 100 characters and no control characters.")
			return
		}
		in.Name = &name
	}
	if _, err := a.db.Exec(r.Context(), `update users set name = $2 where id = $1`, UserID(r.Context()), in.Name); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	a.me(w, r)
}
