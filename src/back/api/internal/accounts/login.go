package accounts

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
)

// login signs in with email and password.
func (a *Accounts) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !httpx.Decode(w, r, &in) {
		return
	}
	var u user
	var hash string
	err := a.db.QueryRow(r.Context(),
		`select id, email, password_hash from users where email = $1`, in.Email).Scan(&u.ID, &u.Email, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		hash = dummyHash()
	} else if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if !checkPassword(hash, in.Password) || u.ID == 0 {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
		return
	}
	if err := a.startSession(r.Context(), w, u.ID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}
