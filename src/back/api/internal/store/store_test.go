package store_test

import (
	"context"
	"io/fs"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/store"
	"gantt-stop-me/api/migrations"
)

// Every boot after the first re-runs Migrate on an already-migrated database.
func TestMigrateSkipsApplied(t *testing.T) {
	db := apptest.DB(t) // already migrated once
	ctx := context.Background()
	if err := store.Migrate(ctx, db, migrations.FS); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(ctx, "select count(*) from schema_migrations").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(names) {
		t.Fatalf("schema_migrations has %d rows, want %d", n, len(names))
	}
}
