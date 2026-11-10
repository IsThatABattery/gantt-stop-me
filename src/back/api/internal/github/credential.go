package github

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/oauth"
	"gantt-stop-me/api/internal/secret"
)

var (
	// ErrNotLinked means the user has no linked GitHub account.
	ErrNotLinked = errors.New("github: account not linked")
	// ErrUnavailable means the user's OAuth token expired and could not be refreshed, such as while GitHub is down, or
	// while it refuses the OAuth App's credentials. It is no verdict on the user's credential, so callers take it as
	// temporary rather than ask the user to link again.
	ErrUnavailable = errors.New("github: credential expired and could not be refreshed")
)

// saveTimeout bounds saving what GitHub answered a refresh, however long the refresh waited before it asked.
const saveTimeout = 10 * time.Second

// Credentials reads users' GitHub credentials, refreshing expiring OAuth tokens with the OAuth App that issued them.
// Do not copy one once it is in use.
type Credentials struct {
	DB           *pgxpool.Pool
	Secrets      *secret.Box // encrypts stored credentials
	Client       *Client
	ClientID     string // OAuth App; linking with OAuth, and refreshing its tokens, is off while either is empty
	ClientSecret string

	// gates lets one refresh of a user's run at a time in this process, and the reads waiting for it hold no
	// pooled connection.
	gates oauth.Gates[int64] // by user id
	// failures holds each user's last failed refresh, so their reads do not all try again meanwhile.
	failures oauth.Backoff[int64] // by user id
}

// oauthConfigured reports whether the OAuth App is set up.
func (c *Credentials) oauthConfigured() bool { return c.ClientID != "" && c.ClientSecret != "" }

// tokenEndpoint is the OAuth App's token endpoint.
func (c *Credentials) tokenEndpoint() *oauth.Endpoint {
	return &oauth.Endpoint{TokenURL: c.Client.Web + "/login/oauth/access_token", ClientID: c.ClientID,
		ClientSecret: c.ClientSecret, HTTP: c.Client.HTTP}
}

// Token returns the user's GitHub access token, decrypted, or ErrNotLinked. An OAuth token that expires within
// oauth.RefreshMargin is refreshed first, by one read of the user's at a time. The stored token comes back as it is,
// for GitHub's 401 to ask the user to link again, when GitHub refuses the refresh token, which is then forgotten, or
// when there is no refresh token or OAuth App to refresh with. When GitHub refuses the refresh another way, as for the
// OAuth App's credentials, which linking again does not mend, or the refresh fails for now, as while GitHub is down,
// the stored token comes back until it expires, then ErrUnavailable does. After such a refresh, the user's reads try
// no refresh for oauth.RefreshBackoff, and get the stored token until it expires, then ErrUnavailable.
func (c *Credentials) Token(ctx context.Context, userID int64) (string, error) {
	l, err := c.read(ctx, c.DB, userID, false)
	if err != nil {
		return "", err
	}
	token, err := c.open(userID, l.sealed)
	if err != nil || !c.due(l) {
		return token, err
	}
	if cause := c.failures.Failed(userID); cause != nil {
		return c.fallback(ctx, userID, token, l.expires, cause)
	}
	return c.refresh(ctx, userID, token, l.expires)
}

// Replaced reports whether the user's credential, as Token reads it now, is another token than token. GitHub retires
// a token as it refreshes it, so a 401 for a replaced token says nothing about the credential in use now.
func (c *Credentials) Replaced(ctx context.Context, userID int64, token string) bool {
	current, err := c.Token(ctx, userID)
	return err == nil && current != token
}

// Fail answers a request whose GitHub call with token, the user's credential, failed with err, as Fail does, except
// that GitHub's 401 for a token the user's credential has since replaced, as another request's refresh does, answers
// 502 github_error: the credential in use now may work, so trying again, not linking again, is what helps.
func (c *Credentials) Fail(w http.ResponseWriter, r *http.Request, userID int64, token string, err error) {
	if ghErr := (*Error)(nil); errors.As(err, &ghErr) && ghErr.Status == http.StatusUnauthorized &&
		c.Replaced(r.Context(), userID, token) {
		// Not %w: Fail answers an error without GitHub's status with 502.
		err = fmt.Errorf("%v, for a token user %d's credential has since replaced", err, userID)
	}
	Fail(w, r, err)
}

// link is the user's stored credential, as read holds it.
type link struct {
	sealed        []byte     // the access token
	expires       *time.Time // when it expires, nil for never
	refreshable   bool       // a refresh token is stored
	sealedRefresh []byte     // the refresh token, read only for update
}

// querier is the pool or a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// read loads the user's stored credential through q, the pool or a transaction, or returns ErrNotLinked. forUpdate
// locks the row until the transaction ends, and loads the refresh token.
func (c *Credentials) read(ctx context.Context, q querier, userID int64, forUpdate bool) (link, error) {
	var l link
	var err error
	if forUpdate {
		err = q.QueryRow(ctx, `select credential_enc, expires_at, refresh_enc from github_links where user_id = $1 for update`,
			userID).Scan(&l.sealed, &l.expires, &l.sealedRefresh)
		l.refreshable = l.sealedRefresh != nil
	} else {
		err = q.QueryRow(ctx, `select credential_enc, expires_at, refresh_enc is not null from github_links where user_id = $1`,
			userID).Scan(&l.sealed, &l.expires, &l.refreshable)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return link{}, ErrNotLinked
	}
	return l, err
}

// due reports whether l's token should be refreshed now: it is due, and a refresh token and the OAuth App to use it
// with are there. Without them the token is used as it is, until GitHub refuses it.
func (c *Credentials) due(l link) bool {
	return oauth.Due(l.expires) && l.refreshable && c.oauthConfigured()
}

// refresh refreshes the user's expiring OAuth token, the stored token, which expires at expires. Reads in this process
// queue at the user's gate before they take a pooled connection, and each finds what the one before it saved; the row
// lock keeps out other api instances' refreshes. GitHub retires the old tokens as it answers, so once asked, a refresh
// is saved even when ctx ends.
func (c *Credentials) refresh(ctx context.Context, userID int64, token string, expires *time.Time) (string, error) {
	unlock, err := c.gates.Wait(ctx, userID)
	if err != nil {
		return c.fallback(ctx, userID, token, expires, err)
	}
	defer unlock()
	// The read before this one may have refreshed, or failed to, while this one waited.
	l, err := c.read(ctx, c.DB, userID, false)
	if errors.Is(err, ErrNotLinked) {
		return "", err
	}
	if err != nil {
		return c.fallback(ctx, userID, token, expires, err)
	}
	if token, err = c.open(userID, l.sealed); err != nil || !c.due(l) {
		return token, err
	}
	expires = l.expires
	if cause := c.failures.Failed(userID); cause != nil {
		return c.fallback(ctx, userID, token, expires, cause)
	}

	tx, err := c.DB.Begin(ctx)
	if err != nil {
		return c.fallback(ctx, userID, token, expires, err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	// Another api instance may hold the row for its own refresh: wait a while for it, then make do.
	if _, err = tx.Exec(ctx, `set local lock_timeout = '5s'`); err == nil {
		l, err = c.read(ctx, tx, userID, true)
	}
	if errors.Is(err, ErrNotLinked) {
		return "", err // unlinked meanwhile
	}
	if err != nil {
		return c.fallback(ctx, userID, token, expires, err)
	}
	if token, err = c.open(userID, l.sealed); err != nil || !c.due(l) {
		return token, err // refreshed, relinked or refused by another instance meanwhile
	}
	expires = l.expires
	refreshToken, err := c.Secrets.Open(l.sealedRefresh)
	if err != nil {
		return "", fmt.Errorf("github: open refresh token of user %d: %w", userID, err)
	}

	tok, err := c.tokenEndpoint().Refresh(context.WithoutCancel(ctx), string(refreshToken))
	// A fresh deadline: the time spent waiting must not cut saving what GitHub answered.
	save, cancel := context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
	defer cancel()
	var oauthErr *oauth.Error
	errors.As(err, &oauthErr)
	switch {
	case oauthErr != nil && oauthErr.Code == "bad_refresh_token": // expired, or used up, as by a lost answer
		log.Printf("github: refresh the credential of user %d: %v; they must link again", userID, err)
		// Forgotten, so later reads use the stored token without asking, and GitHub's 401 for it asks the user to link again.
		if err := commit(save, tx, `update github_links set refresh_enc = null where user_id = $1`, userID); err != nil {
			log.Printf("github: forget the refresh token of user %d: %v", userID, err)
			c.failures.Fail(userID, err)
		}
		return token, nil
	case err != nil:
		// Another refusal, such as incorrect_client_credentials, which asking again gets until the OAuth App is set up
		// right, and linking again does not mend; or GitHub out of reach, its 5xx or its 429.
		log.Printf("github: refresh the credential of user %d: %v", userID, err)
		c.failures.Fail(userID, err)
		return c.fallback(ctx, userID, token, expires, err)
	}
	if err := commit(save, tx, `update github_links set credential_enc = $2, refresh_enc = $3, expires_at = $4 where user_id = $1`,
		userID, c.Secrets.Seal([]byte(tok.AccessToken)), c.sealRefresh(tok), expiry(tok)); err != nil {
		// GitHub retired the stored tokens, so the new one is all there is: the next refresh fails, and the user links again.
		log.Printf("github: save the refreshed credential of user %d: %v", userID, err)
		return tok.AccessToken, nil
	}
	c.failures.Clear(userID)
	return tok.AccessToken, nil
}

// commit runs sql in tx and commits it.
func commit(ctx context.Context, tx pgx.Tx, sql string, args ...any) error {
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// fallback is what a read of the user's token returns when it could not be refreshed, for cause: the stored
// token while it has not expired at expires, else ctx's error once the caller gave up, and else ErrUnavailable.
func (c *Credentials) fallback(ctx context.Context, userID int64, token string, expires *time.Time, cause error) (string, error) {
	if time.Now().Before(*expires) {
		return token, nil
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("github: read the credential of user %d: %w", userID, err)
	}
	return "", fmt.Errorf("%w: user %d: %v", ErrUnavailable, userID, cause) // not %w: the token endpoint's status is no verdict on the credential
}

// open decrypts a stored credential of the user's.
func (c *Credentials) open(userID int64, sealed []byte) (string, error) {
	token, err := c.Secrets.Open(sealed)
	if err != nil {
		return "", fmt.Errorf("github: open credential of user %d: %w", userID, err)
	}
	return string(token), nil
}

// sealRefresh is how tok's refresh token is stored: sealed, or nil for none.
func (c *Credentials) sealRefresh(tok oauth.Token) []byte {
	if tok.RefreshToken == "" {
		return nil
	}
	return c.Secrets.Seal([]byte(tok.RefreshToken))
}

// expiry is how tok's expiry is stored: nil for never.
func expiry(tok oauth.Token) *time.Time {
	if tok.Expires.IsZero() {
		return nil
	}
	return &tok.Expires
}

// save links userID to the GitHub account u with tok, through q, the pool or a transaction, replacing any earlier
// link and its refresh token and expiry, and forgets a failed refresh of the old one. Project access checked with the
// old credential goes stale, so the next look checks it with the new one; the rows stay, since they also list the
// projects the user opened.
func (l *Links) save(ctx context.Context, q querier, userID int64, u User, tok oauth.Token, kind string) error {
	_, err := q.Exec(ctx, `
		with stale as (update project_members set checked_at = '-infinity' where user_id = $1)
		insert into github_links (user_id, github_id, login, credential_enc, kind, refresh_enc, expires_at)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (user_id) do update set github_id = excluded.github_id, login = excluded.login,
			credential_enc = excluded.credential_enc, kind = excluded.kind, refresh_enc = excluded.refresh_enc,
			expires_at = excluded.expires_at, linked_at = now()`,
		userID, u.ID, u.Login, l.Secrets.Seal([]byte(tok.AccessToken)), kind, l.sealRefresh(tok), expiry(tok))
	if err == nil {
		l.failures.Clear(userID)
	}
	return err
}
