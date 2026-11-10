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

// me returns the signed-in user, their display name or null, their linked GitHub login, or null for github, and
// whether they have a password, which an account made by signing in with GitHub has not until they set one.
func (a *Accounts) me(w http.ResponseWriter, r *http.Request) {
	var out struct {
		user
		Name        *string     `json:"name"`
		GitHub      *githubLink `json:"github"`
		HasPassword bool        `json:"hasPassword"`
	}
	var login, kind *string
	err := a.db.QueryRow(r.Context(), `
		select u.id, u.email, u.name, u.password_hash is not null, g.login, g.kind
		from users u left join github_links g on g.user_id = u.id
		where u.id = $1`, UserID(r.Context())).Scan(&out.ID, &out.Email, &out.Name, &out.HasPassword, &login, &kind)
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
		// Format characters and line separators are invisible or reorder text, so they could hide or fake a name.
		invisible := func(r rune) bool { return unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) }
		if n := utf8.RuneCountInString(name); n < 1 || n > 100 || strings.IndexFunc(name, invisible) >= 0 {
			httpx.Error(w, http.StatusBadRequest, "invalid_name",
				"A name needs 1 to 100 characters, not all spaces, and no control, format or line separator characters.")
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
