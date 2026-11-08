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
	ok := checkPassword(hash, in.Password) && u.ID != 0
	if ok {
		// ok turns false if the password changed while it was being checked.
		if ok, err = a.startSession(r.Context(), w, u.ID, hash); err != nil {
			httpx.Internal(w, r, err)
			return
		}
	}
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}
