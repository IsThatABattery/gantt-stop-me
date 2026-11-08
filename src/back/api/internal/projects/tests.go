package projects

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/tasktests"
)

// TakeTests takes the Tests section of body, as GitHub has task id's issue, into the task's card; every read of an
// issue body calls it. A section the card last saw changes nothing, but clears tests_error. Any other is parsed:
// valid tests replace the card's, and the card has now seen the section; otherwise the card keeps its tests and
// records why in tests_error, and the next read tries again. It reports whether the card's tests or tests_error
// changed.
func TakeTests(ctx context.Context, tx pgx.Tx, id int64, body string) (changed bool, err error) {
	section := tasktests.Section(body)
	var synced *string
	if err := tx.QueryRow(ctx, `select tests_synced from tasks where id = $1`, id).Scan(&synced); err != nil {
		return false, err
	}
	if synced != nil && *synced == section {
		_, err := tx.Exec(ctx, `update tasks set tests_error = null where id = $1 and tests_error is not null`, id)
		return false, err
	}
	tests, err := tasktests.Parse(body)
	if err == nil {
		err = tasktests.Validate(tests)
	}
	if err == nil && strings.ContainsRune(section, 0) { // in a line that is no test's, which Postgres cannot store
		err = errors.New("The Tests section holds a NUL character.")
	}
	if err != nil {
		err = tx.QueryRow(ctx, `update tasks set tests_error = $2 where id = $1 returning old.tests_error is distinct from new.tests_error`,
			id, err.Error()).Scan(&changed)
		return changed, err
	}
	err = tx.QueryRow(ctx, `
		update tasks set tests = $2, tests_synced = $3, tests_error = null where id = $1 returning old.tests is distinct from new.tests`,
		id, tests, section).Scan(&changed)
	return changed, err
}
