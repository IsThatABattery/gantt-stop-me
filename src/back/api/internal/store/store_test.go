package store_test

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

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

// 005 first shipped without projects.backfill_pending, and databases that ran it record 005 as applied all the same.
func TestMigrateAfterTheFirst005(t *testing.T) {
	db := apptest.EmptyDB(t)
	ctx := context.Background()
	old := fstest.MapFS{"005_task_details.sql": {Data: []byte(`
		alter table tasks add column body text not null default '', add column labels jsonb not null default '[]';
		update projects set synced_at = 'epoch', issues_etag = null where synced_at is not null;`)}}
	for _, name := range []string{"001_accounts.sql", "002_github.sql", "003_board.sql", "004_providers.sql"} {
		data, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		old[name] = &fstest.MapFile{Data: data}
	}
	if err := store.Migrate(ctx, db, old); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into projects (github_repo_id, owner, name, synced_at) values (1, 'octo', 'hello', now())`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	var pending bool
	if err := db.QueryRow(ctx, `select backfill_pending from projects`).Scan(&pending); err != nil || !pending {
		t.Fatalf("backfill pending %v (%v), want the label backfill set", pending, err)
	}
}
