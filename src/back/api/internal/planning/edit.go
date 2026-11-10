package planning

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// edit changes the doc's sections as a person wrote them, unless the plan changed since they loaded its version.
func (p *Planning) edit(w http.ResponseWriter, r *http.Request) {
	pr, token, ok := p.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Doc     []change `json:"doc"`
		Version int      `json:"version"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if !p.change(w, r, pr.ID, token, func(ctx context.Context, tx pgx.Tx, pl plan) error {
		if err := waiting(pl, "briefing", "needs_review"); err != nil {
			return err
		}
		if in.Version != pl.Version {
			return errConflict
		}
		doc, err := apply(pl.Doc, in.Doc)
		if err != nil {
			return &refusal{http.StatusBadRequest, "invalid_doc", "In the doc, " + err.Error() + "."}
		}
		_, err = tx.Exec(ctx, `update feature_plans set doc = $2 where id = $1`, pl.ID, doc)
		return err
	}) {
		return
	}
	p.show(w, r, http.StatusOK, pr.ID, planID(r))
}
