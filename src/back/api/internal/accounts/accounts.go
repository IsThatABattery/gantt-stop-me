// Package accounts owns users, passwords and sessions.
package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
)

// Accounts serves the auth and /api/me endpoints and guards every other signed-in route.
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
	mux.HandleFunc("GET /api/me", a.Require(a.me))
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

// Require runs h only for a request with a valid session, with the user ID in its context.
func (a *Accounts) Require(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userID int64
		c, err := r.Cookie(SessionCookie)
		if err == nil {
			err = a.db.QueryRow(r.Context(),
				`select user_id from sessions where id = $1 and expires_at > now()`,
				hashToken(c.Value)).Scan(&userID)
		}
		if errors.Is(err, http.ErrNoCookie) || errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "Sign in first.")
			return
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, userID)))
	}
}

// startSession stores a new session for userID and sets its cookie on w.
func (a *Accounts) startSession(ctx context.Context, w http.ResponseWriter, userID int64) error {
	token := rand.Text()
	_, err := a.db.Exec(ctx, `
		with expired as (delete from sessions where user_id = $2 and expires_at <= now())
		insert into sessions (id, user_id, expires_at) values ($1, $2, $3)`,
		hashToken(token), userID, time.Now().Add(sessionTTL))
	if err != nil {
		return err
	}
	http.SetCookie(w, a.cookie(token, int(sessionTTL.Seconds())))
	return nil
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
