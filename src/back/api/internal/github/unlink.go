package github

import (
	"net/http"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// unlink forgets the user's GitHub account. It succeeds even when none is linked.
func (l *Links) unlink(w http.ResponseWriter, r *http.Request) {
	if _, err := l.DB.Exec(r.Context(), `delete from github_links where user_id = $1`, accounts.UserID(r.Context())); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
