package settings

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// delete removes one of the user's providers, unless a project's automation or the user's planning settings use it:
// that would turn them off without turning them off properly, so the user switches providers or turns them off first.
func (s *Settings) delete(w http.ResponseWriter, r *http.Request) {
	tag, err := s.db.Exec(r.Context(), `delete from providers where id = $1 and user_id = $2`,
		providerID(r), accounts.UserID(r.Context()))
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
		httpx.Error(w, http.StatusConflict, "provider_in_use",
			"A project's automation or your planning settings use this provider. Pick another provider there, or turn it off, first.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusNotFound, "not_found", "Provider not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
