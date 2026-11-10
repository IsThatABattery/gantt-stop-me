package github

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/oauth"
)

var (
	// errDenied means the user cancelled on GitHub.
	errDenied = errors.New("github: the user cancelled on GitHub")
	// errTaken means another user is linked to the GitHub account with OAuth, so it signs in as them.
	errTaken = errors.New("github: another user is linked to the GitHub account with OAuth")
	// errPasswordRequired means a user with no password tried to link another GitHub account than the one they sign
	// in with.
	errPasswordRequired = errors.New("github: a user with no password tried to link another GitHub account")
)

// callback finishes the flow the state is for: signing in, which has no session yet, or else linking, which needs
// the session that started it.
func (l *Links) callback(w http.ResponseWriter, r *http.Request) {
	l.nextCookie().Clear(w) // only a sign-in reads it, and only this once
	if l.stateCookie().Flow(r) == oauth.SignIn {
		l.signInCallback(w, r)
		return
	}
	l.Accounts.RequireOr(l.signedOut, l.linkCallback)(w, r)
}

// linkCallback finishes linking with OAuth and sends the browser back to Settings with the outcome.
func (l *Links) linkCallback(w http.ResponseWriter, r *http.Request) {
	l.stateCookie().Clear(w)
	outcome := "linked"
	switch err := l.finishLink(r); {
	case errors.Is(err, errDenied):
		outcome = "denied"
	case errors.Is(err, errTaken):
		outcome = "taken"
	case errors.Is(err, errPasswordRequired):
		outcome = "password_required"
	case err != nil:
		// Never with the code, the tokens or the client secret.
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		outcome = "error"
	}
	l.settings(w, r, outcome)
}

// finishLink links the session's user to the GitHub account the callback authorizes, unless another user is linked
// to it with OAuth, which would leave sign-in two users to choose from, or the user has no password and is linked
// to another GitHub account, their only sign-in. It saves nothing on failure.
func (l *Links) finishLink(r *http.Request) error {
	session, _ := r.Cookie(accounts.SessionCookie) // RequireOr guarantees it
	u, tok, err := l.authorized(r, oauth.Link, session.Value)
	if err != nil {
		return err
	}
	ctx, userID := r.Context(), accounts.UserID(r.Context())
	return pgx.BeginFunc(ctx, l.DB, func(tx pgx.Tx) error {
		// Takes turns with the account's sign-ins and other links, so none finds it unlinked while this links it.
		if err := lockGitHubAccount(ctx, tx, u.ID); err != nil {
			return err
		}
		var taken bool
		if err := tx.QueryRow(ctx, `select exists (select from github_links where github_id = $1 and kind = 'oauth' and user_id <> $2)`,
			u.ID, userID).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return errTaken
		}
		// The row lock keeps the user's other links out until this one saves. No key: foreign-key inserts that refer to
		// the user, such as a project member's, need not wait; a new session's does, since insertSession takes for share.
		var hasPassword bool
		var current *int64 // the GitHub account linked now, nil for none
		if err := tx.QueryRow(ctx, `
			select u.password_hash is not null, g.github_id
			from users u left join github_links g on g.user_id = u.id
			where u.id = $1 for no key update of u`, userID).Scan(&hasPassword, &current); err != nil {
			return err
		}
		if !hasPassword && current != nil && *current != u.ID {
			return errPasswordRequired
		}
		return l.save(ctx, tx, userID, u, tok, "oauth")
	})
}

// lockGitHubAccount holds, until tx ends, the lock that one GitHub account's sign-ins and links take turns under.
func lockGitHubAccount(ctx context.Context, tx pgx.Tx, id int64) error {
	_, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('github sign-in'), hashtext($1::bigint::text))`, id)
	return err
}

// authorized checks the callback's state, issued for a flow of kind and session, then GitHub's error, exchanges the
// code and loads the GitHub user it authorizes. Only a valid state shows the answer came from GitHub for this
// browser, so nothing else counts without one.
func (l *Links) authorized(r *http.Request, kind oauth.Kind, session string) (User, oauth.Token, error) {
	q := r.URL.Query()
	if err := l.stateCookie().Verify(r, kind, session); err != nil {
		if e := q.Get("error"); e != "" {
			return User{}, oauth.Token{}, fmt.Errorf("%w; the callback also had error %q", err, e) // quoted: anyone can send it
		}
		return User{}, oauth.Token{}, err
	}
	if e := q.Get("error"); e == "access_denied" {
		return User{}, oauth.Token{}, errDenied
	} else if e != "" {
		return User{}, oauth.Token{}, l.refused(kind, e, q.Get("error_description"))
	}
	tok, err := l.tokenEndpoint().Exchange(r.Context(), q.Get("code"), l.PublicURL+callbackPath)
	if oauthErr := (*oauth.Error)(nil); errors.As(err, &oauthErr) {
		return User{}, oauth.Token{}, l.refused(kind, oauthErr.Code, oauthErr.Description)
	}
	if err != nil {
		return User{}, oauth.Token{}, fmt.Errorf("exchange the code: %w", err)
	}
	u, err := l.Client.User(r.Context(), tok.AccessToken)
	if err != nil {
		return User{}, oauth.Token{}, fmt.Errorf("load the GitHub user: %w", err)
	}
	return u, tok, nil
}

// refused explains GitHub refusing a flow of kind with error code and its description, which may be "". Both are
// quoted, since the callback's query can say anything, so a newline in them cannot forge a log line.
func (l *Links) refused(kind oauth.Kind, code, description string) error {
	flow := "link"
	if kind == oauth.SignIn {
		flow = "sign-in"
	}
	msg := fmt.Sprintf("GitHub refused the %s: %q", flow, code)
	if description != "" {
		msg += fmt.Sprintf(": %q", description)
	}
	if code == "redirect_uri_mismatch" {
		msg += "; the OAuth App's callback URL must be " + l.PublicURL + callbackPath
	}
	return errors.New(msg)
}
