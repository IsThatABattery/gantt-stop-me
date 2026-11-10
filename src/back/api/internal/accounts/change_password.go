package accounts

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
)

var (
	errWrongPassword  = errors.New("wrong password")
	errReauthRequired = errors.New("no password, and the session is not recent")
)

// changePassword replaces the password and signs out every other session. For an account with none, as one made by
// signing in with GitHub, it sets the first one, which needs no current password but a recent session. Such an
// account signs in only with GitHub and has no password to confirm, so only a session it started moments ago is
// recent: proof its user holds the GitHub account, which an older, perhaps stolen, one is not.
func (a *Accounts) changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if len(in.NewPassword) < minPasswordLen {
		httpx.Error(w, http.StatusBadRequest, "weak_password", "Password must be at least 8 characters.")
		return
	}
	ctx, userID := r.Context(), UserID(r.Context())
	c, _ := r.Cookie(SessionCookie) // Require guarantees it
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		// The row lock serializes changes, so a concurrent one checks against the hash this one set.
		var hash *string
		if err := tx.QueryRow(ctx, `select password_hash from users where id = $1 for update`, userID).Scan(&hash); err != nil {
			return err
		}
		if hash != nil && !checkPassword(*hash, in.CurrentPassword) {
			return errWrongPassword
		}
		recent, err := RecentSession(tx, r)
		if err != nil {
			return err // ErrSignedOut: a concurrent change signed this session out
		}
		if hash == nil && !recent {
			return errReauthRequired
		}
		if _, err := tx.Exec(ctx, `update users set password_hash = $2 where id = $1`, userID, hashPassword(in.NewPassword)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `delete from sessions where user_id = $1 and id <> $2`, userID, hashToken(c.Value))
		return err
	})
	switch {
	case errors.Is(err, errWrongPassword):
		httpx.Error(w, http.StatusForbidden, "wrong_password", "Current password is incorrect.")
	case errors.Is(err, errReauthRequired):
		httpx.Error(w, http.StatusForbidden, "reauth_required", "Sign in with GitHub again, then set your password.")
	case errors.Is(err, ErrSignedOut), errors.Is(err, pgx.ErrNoRows):
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "Sign in first.")
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
