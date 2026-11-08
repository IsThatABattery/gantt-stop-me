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
// project's lock throughout, so a sync cannot take the section from before the write. It first takes GitHub's
// section, and when that changes the card, or the card's tests are no longer the base the client loaded, such as
// after a sync took a GitHub edit, it answers 409 with the card as it now is instead.
func (b *Board) setTests(w http.ResponseWriter, r *http.Request) {
	p, token, ok := b.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Tests []tasktests.Test `json:"tests"`
		Base  []tasktests.Test `json:"base"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	switch { // left out, which must neither clear the card nor skip the clash check
	case in.Tests == nil:
		httpx.Error(w, http.StatusBadRequest, "invalid_tests", "Send the tests, [] for none.")
		return
	case in.Base == nil:
		httpx.Error(w, http.StatusBadRequest, "invalid_tests", "Send base, the tests as you last loaded them.")
		return
	}
	if err := tasktests.Validate(in.Tests); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_tests", err.Error())
		return
	}
	// As GitHub reads them back, so the card never holds tests its issue shows otherwise. Checked again, because a
	// name that is only a [name] pin reads back as no name.
	tests, _ := tasktests.Parse(tasktests.Render(in.Tests))
	if err := tasktests.Validate(tests); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_tests", err.Error())
		return
	}
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
	var now []tasktests.Test
	var testsError *string
	var stale bool
	if err == nil {
		err = tx.QueryRow(ctx, `select tests, tests_error, tests is distinct from $2 from tasks where id = $1`, id, in.Base).
			Scan(&now, &testsError, &stale)
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if changed || stale {
		// The body too, so a client that edits the description next sends GitHub's section, not the one it had.
		_, err := tx.Exec(ctx, `update tasks set body = $2 where id = $1`, id, is.Body)
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusConflict, map[string]any{
			"error": httpx.ErrorBody{Code: "tests_changed", Message: "The tests changed on GitHub since you loaded them. They are shown as they are now."},
			"tests": now, "testsError": testsError, "body": is.Body,
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
	// updated_at, as an edit sets it: a sync that read the issue before this then leaves the card alone.
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
	// The body too, so a client that edits the description next keeps these tests.
	httpx.JSON(w, http.StatusOK, map[string]any{"tests": tests, "body": is.Body})
}
