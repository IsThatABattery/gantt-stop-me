package accounts

import (
	"net/http"

	"gantt-stop-me/api/internal/httpx"
)

// logout ends the current session. It succeeds even without one.
func (a *Accounts) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		if _, err := a.db.Exec(r.Context(), `delete from sessions where id = $1`, hashToken(c.Value)); err != nil {
			httpx.Internal(w, r, err)
			return
		}
	}
	http.SetCookie(w, a.cookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}
