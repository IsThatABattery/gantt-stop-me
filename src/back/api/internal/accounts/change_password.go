package accounts

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
)

// changePassword replaces the password and signs out every other session.
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
	var hash string
	if err := a.db.QueryRow(ctx, `select password_hash from users where id = $1`, userID).Scan(&hash); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if !checkPassword(hash, in.CurrentPassword) {
		httpx.Error(w, http.StatusForbidden, "wrong_password", "Current password is incorrect.")
		return
	}
	c, _ := r.Cookie(SessionCookie) // Require guarantees it
	err := pgx.BeginFunc(ctx, a.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `update users set password_hash = $2 where id = $1`, userID, hashPassword(in.NewPassword)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `delete from sessions where user_id = $1 and id <> $2`, userID, hashToken(c.Value))
		return err
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
