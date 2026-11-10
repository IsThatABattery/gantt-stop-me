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
	var hash *string // nil for an account with no password, as one made by signing in with GitHub
	err := a.db.QueryRow(r.Context(),
		`select id, email, password_hash from users where email = $1`, in.Email).Scan(&u.ID, &u.Email, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.Internal(w, r, err)
		return
	}
	var check string
	if hash != nil {
		check = *hash
	} else {
		// An unknown email, or an account with no password: checked all the same, so it takes as long as a wrong password.
		check = dummyHash()
	}
	ok := checkPassword(check, in.Password) && hash != nil
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
