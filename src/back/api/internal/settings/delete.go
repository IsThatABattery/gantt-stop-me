package settings

import (
	"net/http"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// delete removes one of the user's providers.
func (s *Settings) delete(w http.ResponseWriter, r *http.Request) {
	tag, err := s.db.Exec(r.Context(), `delete from providers where id = $1 and user_id = $2`,
		providerID(r), accounts.UserID(r.Context()))
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
