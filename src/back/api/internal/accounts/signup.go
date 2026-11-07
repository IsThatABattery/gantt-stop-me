package accounts

import (
	"errors"
	"net/http"
	"net/mail"

	"github.com/jackc/pgx/v5/pgconn"

	"gantt-stop-me/api/internal/httpx"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// signup creates an account and signs it in.
func (a *Accounts) signup(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !httpx.Decode(w, r, &in) {
		return
	}
	if addr, err := mail.ParseAddress(in.Email); err != nil || addr.Address != in.Email || len(in.Email) > 254 {
		httpx.Error(w, http.StatusBadRequest, "invalid_email", "Enter a valid email address.")
		return
	}
	if len(in.Password) < minPasswordLen {
		httpx.Error(w, http.StatusBadRequest, "weak_password", "Password must be at least 8 characters.")
		return
	}

	var u user
	err := a.db.QueryRow(r.Context(),
		`insert into users (email, password_hash) values ($1, $2) returning id, email`,
		in.Email, hashPassword(in.Password)).Scan(&u.ID, &u.Email)
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23505" {
		httpx.Error(w, http.StatusConflict, "email_taken", "An account with this email already exists.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if err := a.startSession(r.Context(), w, u.ID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, u)
}
