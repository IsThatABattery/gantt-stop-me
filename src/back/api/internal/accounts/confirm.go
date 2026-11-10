package accounts

import (
	"net/http"

	"gantt-stop-me/api/internal/httpx"
)

// confirm checks the user's password and, when it is right, marks the current session as having confirmed it, which
// makes it recent again: see RecentSession. A wrong password takes as long as a right one, as at login.
func (a *Accounts) confirm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	ctx := r.Context()
	var hash *string
	if err := a.db.QueryRow(ctx, `select password_hash from users where id = $1`, UserID(ctx)).Scan(&hash); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if hash == nil {
		httpx.Error(w, http.StatusConflict, "password_required", "This account has no password. Sign in with GitHub again instead.")
		return
	}
	if !checkPassword(*hash, in.Password) {
		httpx.Error(w, http.StatusForbidden, "wrong_password", "Your password is wrong.")
		return
	}
	// The hash is not checked again: a change from another session signs this one out whenever it commits, and one
	// from this session needed the password too.
	c, _ := r.Cookie(SessionCookie) // Require guarantees it
	tag, err := a.db.Exec(ctx, `update sessions set confirmed_at = now() where id = $1`, hashToken(c.Value))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "Sign in first.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
