package automation

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// show answers the project's automation settings, or {"enabled": false} while it is off.
func (a *Automation) show(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	s, err := a.settings(r.Context(), a.DB, p.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.JSON(w, http.StatusOK, map[string]bool{"enabled": false})
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}
