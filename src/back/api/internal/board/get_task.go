package board

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/tasktests"
)

// getTask returns a task with its issue's cached body and labels and the card's tests, after a sync with GitHub if
// one is due.
func (b *Board) getTask(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Read)
	if !ok || !b.syncIfDue(w, r, p, token) {
		return
	}
	var out struct {
		task
		Body   string `json:"body"`
		Labels []struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"labels"` // not github.Label, whose id only the sync needs
		Tests      []tasktests.Test `json:"tests"`
		TestsError *string          `json:"testsError"`
	}
	var err error
	out.task, err = b.scanTask(b.DB.QueryRow(r.Context(), `
		select `+taskColumns+`, t.body, t.labels, t.tests, t.tests_error from tasks t where t.id = $1 and t.project_id = $2`,
		taskID(r), p.ID), p, &out.Body, &out.Labels, &out.Tests, &out.TestsError)
	if errors.Is(err, pgx.ErrNoRows) {
		taskNotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
