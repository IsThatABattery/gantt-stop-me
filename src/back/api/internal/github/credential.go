package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/secret"
)

// ErrNotLinked means the user has no linked GitHub account.
var ErrNotLinked = errors.New("github: account not linked")

// Token returns the user's GitHub credential, decrypted, or ErrNotLinked.
func Token(ctx context.Context, db *pgxpool.Pool, box *secret.Box, userID int64) (string, error) {
	var sealed []byte
	err := db.QueryRow(ctx, `select credential_enc from github_links where user_id = $1`, userID).Scan(&sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotLinked
	}
	if err != nil {
		return "", err
	}
	token, err := box.Open(sealed)
	if err != nil {
		return "", fmt.Errorf("github: open credential of user %d: %w", userID, err)
	}
	return string(token), nil
}

// save links userID to the GitHub account u, replacing any earlier link.
// Project access checked with the old credential goes stale, so the next look checks it with the new one; the rows
// stay, since they also list the projects the user opened.
func (l *Links) save(ctx context.Context, userID int64, u User, token, kind string) error {
	_, err := l.DB.Exec(ctx, `
		with stale as (update project_members set checked_at = '-infinity' where user_id = $1)
		insert into github_links (user_id, github_id, login, credential_enc, kind) values ($1, $2, $3, $4, $5)
		on conflict (user_id) do update set github_id = excluded.github_id, login = excluded.login,
			credential_enc = excluded.credential_enc, kind = excluded.kind, linked_at = now()`,
		userID, u.ID, u.Login, l.Secrets.Seal([]byte(token)), kind)
	return err
}
