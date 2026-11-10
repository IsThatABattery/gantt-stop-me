package github

import (
	"net/http"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// unlink forgets the user's GitHub account, and with it their access to every project. It succeeds even when none is
// linked, but not for a user with no password, who could then not sign in.
func (l *Links) unlink(w http.ResponseWriter, r *http.Request) {
	ctx, userID := r.Context(), accounts.UserID(r.Context())
	if !l.passwordSet(w, r, "Set a password before unlinking GitHub, so you can still sign in.") {
		return
	}
	if _, err := l.DB.Exec(ctx, `
		with forget as (delete from project_members where user_id = $1)
		delete from github_links where user_id = $1`, userID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// passwordSet reports whether the signed-in user has a password, else answers 409 password_required with message and
// returns false. A user without one signs in only with their OAuth link, so it must stay. Nothing removes a password
// once set, so one there now is still there once the link changes.
func (l *Links) passwordSet(w http.ResponseWriter, r *http.Request, message string) bool {
	var set bool
	if err := l.DB.QueryRow(r.Context(), `select password_hash is not null from users where id = $1`,
		accounts.UserID(r.Context())).Scan(&set); err != nil {
		httpx.Internal(w, r, err)
		return false
	}
	if !set {
		httpx.Error(w, http.StatusConflict, "password_required", message)
	}
	return set
}
