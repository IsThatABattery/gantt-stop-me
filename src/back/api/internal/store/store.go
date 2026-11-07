// Package store opens the Postgres pool and applies migrations.
package store

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Open connects to Postgres and checks the connection works.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Migrate applies, in name order, every *.sql file in fsys not yet recorded in schema_migrations.
// Each file runs in its own transaction. An advisory lock stops concurrent callers from racing.
func Migrate(ctx context.Context, db *pgxpool.Pool, fsys fs.FS) error {
	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "select pg_advisory_lock(1)"); err != nil {
		return err
	}
	defer conn.Exec(context.WithoutCancel(ctx), "select pg_advisory_unlock(1)")

	if _, err := conn.Exec(ctx, `create table if not exists schema_migrations (
		version    text primary key,
		applied_at timestamptz not null default now()
	)`); err != nil {
		return err
	}
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		sql, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, "insert into schema_migrations (version) values ($1) on conflict do nothing", name)
			if err != nil || tag.RowsAffected() == 0 {
				return err // already applied
			}
			_, err = tx.Exec(ctx, string(sql))
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}
