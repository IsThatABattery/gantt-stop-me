// Package accounts owns users, passwords and sessions.
package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/httpx"
)

// SessionCookie is the name of the cookie that holds the session token.
const SessionCookie = "session"

const (
	sessionTTL     = 30 * 24 * time.Hour
	minPasswordLen = 8
	// freshSession is how long a session stays recent after it starts or confirms its user's password. Only a recent
	// one adds a way to sign in, a first password or an OAuth link: an older one may be stolen, and the way it added
	// would outlast a password change.
	freshSession = 10 * time.Minute
)

// FreshSession is how long a session stays recent after it starts or confirms its user's password.
const FreshSession = freshSession

// Accounts serves the auth endpoints, /api/me and /api/me/password, and guards every other signed-in route.
type Accounts struct {
	db     *pgxpool.Pool
	secure bool // set Secure on cookies; browsers drop Secure cookies on plain http
}

// New returns Accounts for an app served at publicURL.
func New(db *pgxpool.Pool, publicURL string) *Accounts {
	return &Accounts{db: db, secure: strings.HasPrefix(publicURL, "https://")}
}

// Register adds the accounts routes to mux.
func (a *Accounts) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/signup", a.signup)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("POST /api/auth/confirm", a.Require(a.confirm))
	mux.HandleFunc("GET /api/me", a.Require(a.me))
	mux.HandleFunc("PATCH /api/me", a.Require(a.editMe))
	mux.HandleFunc("PATCH /api/me/password", a.Require(a.changePassword))
}

type user struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type ctxKey struct{}

// UserID returns the signed-in user's ID inside a handler wrapped by Require.
func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(ctxKey{}).(int64)
	return id
}

// ErrSignedOut means the request has no valid session.
var ErrSignedOut = errors.New("accounts: no valid session")

// Require runs h only for a request with a valid session, with the user ID in its context.
func (a *Accounts) Require(h http.HandlerFunc) http.HandlerFunc {
	return a.RequireOr(func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, ErrSignedOut) {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "Sign in first.")
			return
		}
		httpx.Internal(w, r, err)
	}, h)
}

// RequireOr runs h as Require does, and refused for every request Require would refuse, with ErrSignedOut or the
// database's error, for routes a browser navigates to, which answer with a page rather than JSON.
func (a *Accounts) RequireOr(refused func(http.ResponseWriter, *http.Request, error), h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userID int64
		c, err := r.Cookie(SessionCookie)
		if err == nil {
			err = a.db.QueryRow(r.Context(),
				`select user_id from sessions where id = $1 and expires_at > now()`,
				hashToken(c.Value)).Scan(&userID)
		}
		if errors.Is(err, http.ErrNoCookie) || errors.Is(err, pgx.ErrNoRows) {
			err = ErrSignedOut
		}
		if err != nil {
			refused(w, r, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, userID)))
	}
}

// Querier is a pool or a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// RecentSession reports, reading with q, whether the session of r, which Require let through, started or confirmed
// its user's password in the last freshSession, 10 minutes, by the database's clock. It returns ErrSignedOut once the
// session is gone, as when a password change signed it out.
func RecentSession(q Querier, r *http.Request) (bool, error) {
	return RecentSessionWithin(q, r, freshSession)
}

// RecentSessionWithin is RecentSession with another window, for a flow that a recent session started and that may
// end later, such as an OAuth link finishing at its callback.
func RecentSessionWithin(q Querier, r *http.Request, within time.Duration) (bool, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return false, ErrSignedOut
	}
	var recent bool
	err = q.QueryRow(r.Context(), `
		select greatest(created_at, confirmed_at) > now() - make_interval(secs => $3) from sessions where id = $1 and user_id = $2`,
		hashToken(c.Value), UserID(r.Context()), within.Seconds()).Scan(&recent)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrSignedOut
	}
	return recent, err
}

// startSession stores a new session for userID and sets its cookie on w. It reports false, and starts none, if the
// user's password hash is no longer hash, nil for no password: the password changed after it was checked.
func (a *Accounts) startSession(ctx context.Context, w http.ResponseWriter, userID int64, hash *string) (bool, error) {
	return a.insertSession(ctx, w, userID, true, hash)
}

// SignIn starts a session for userID, who proved who they are some other way than with their password, such as with
// their linked GitHub account, and sets its cookie on w, as logging in does. No password was checked, so a password
// change does not stop it.
func (a *Accounts) SignIn(ctx context.Context, w http.ResponseWriter, userID int64) error {
	ok, err := a.insertSession(ctx, w, userID, false, nil)
	if err == nil && !ok {
		err = fmt.Errorf("accounts: start a session: no user %d", userID)
	}
	return err
}

// insertSession stores a new session for userID and sets its cookie on w, unless checkHash is set and the user's
// password hash is no longer hash, or the user is gone.
func (a *Accounts) insertSession(ctx context.Context, w http.ResponseWriter, userID int64, checkHash bool, hash *string) (bool, error) {
	token := rand.Text()
	// for share waits out a password change in progress, then rechecks against the hash it set;
	// a change that starts after this waits instead, and deletes this session.
	tag, err := a.db.Exec(ctx, `
		with expired as (delete from sessions where user_id = $2 and expires_at <= now())
		insert into sessions (id, user_id, expires_at)
		select $1, id, $3 from users where id = $2 and (not $4 or password_hash is not distinct from $5) for share`,
		hashToken(token), userID, time.Now().Add(sessionTTL), checkHash, hash)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	http.SetCookie(w, a.cookie(token, int(sessionTTL.Seconds())))
	return true, nil
}

// cookie builds the session cookie; maxAge < 0 deletes it.
func (a *Accounts) cookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// hashToken is how a session token is stored, so a database leak does not leak sessions.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
