package accounts

import (
	"net/http"

	"gantt-stop-me/api/internal/httpx"
)

// me returns the signed-in user.
func (a *Accounts) me(w http.ResponseWriter, r *http.Request) {
	var u user
	err := a.db.QueryRow(r.Context(), `select id, email from users where id = $1`, UserID(r.Context())).Scan(&u.ID, &u.Email)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}
