package planning

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// list answers the project's plans, newest first, without their messages and proposals.
func (p *Planning) list(w http.ResponseWriter, r *http.Request) {
	pr, _, ok := p.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	rows, _ := p.DB.Query(r.Context(), // CollectRows returns Query's error
		`select `+planColumns+` from feature_plans fp where fp.project_id = $1 order by fp.id desc`, pr.ID)
	plans, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (plan, error) { return scanPlan(row) })
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string][]plan{"plans": plans})
}
