package board

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
	"gantt-stop-me/api/internal/tasktests"
)

// setTests replaces a card's tests, writing them to its issue's Tests section on GitHub first. It holds the
// project's lock throughout, so a sync cannot take the section from before the write. When GitHub's section changed
// since the card last saw it, and taking it changes the card, it answers 409 with the card as it now is instead.
func (b *Board) setTests(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Tests []tasktests.Test `json:"tests"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Tests == nil { // left out, which must not clear the card
		httpx.Error(w, http.StatusBadRequest, "invalid_tests", "Send the tests, [] for none.")
		return
	}
	if err := tasktests.Validate(in.Tests); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_tests", err.Error())
		return
	}
	// As GitHub reads them back, so the card never holds tests its issue shows otherwise.
	tests, _ := tasktests.Parse(tasktests.Render(in.Tests))
	unlock, err := projects.Serialize(r.Context(), p.ID)
	if err != nil {
		return // the client gave up waiting
	}
	defer unlock()
	// Once GitHub has the edit, finish saving it even if the browser goes away.
	ctx := context.WithoutCancel(r.Context())
	tx, err := b.DB.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer tx.Rollback(ctx)
	if err := projects.Lock(ctx, tx, p.ID); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	var id int64
	var number int
	err = tx.QueryRow(ctx, `select id, issue_number from tasks where id = $1 and project_id = $2 for update`, taskID(r), p.ID).
		Scan(&id, &number)
	if errors.Is(err, pgx.ErrNoRows) {
		taskNotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	is, err := b.GitHub.Issue(ctx, token, p.RepoID, number)
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	changed, err := projects.TakeTests(ctx, tx, id, is.Body)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if changed {
		var now []tasktests.Test
		var testsError *string
		err := tx.QueryRow(ctx, `select tests, tests_error from tasks where id = $1`, id).Scan(&now, &testsError)
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusConflict, map[string]any{
			"error": httpx.ErrorBody{Code: "tests_changed", Message: "The tests changed on GitHub since you loaded them. They are shown as they are now."},
			"tests": now, "testsError": testsError,
		})
		return
	}
	body := tasktests.Replace(is.Body, tests)
	if !validBody(w, body) {
		return
	}
	is, err = b.GitHub.UpdateIssue(ctx, token, p.RepoID, number, map[string]string{"body": body})
	if err != nil {
		github.Fail(w, r, err)
		return
	}
	// updated_at, as an edit sets it: a sync or run plan that read the issue before this then leaves the card alone.
	_, err = tx.Exec(ctx, `
		update tasks set tests = $2, tests_synced = $3, tests_error = null, body = $4, updated_at = clock_timestamp() where id = $1`,
		id, tests, tasktests.Section(is.Body), is.Body)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tests": tests})
}
