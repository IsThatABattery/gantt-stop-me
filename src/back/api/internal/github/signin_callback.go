package github

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/oauth"
)

var (
	// errAmbiguous means more than one user is linked to the GitHub account, so none can be told to be the one. Only
	// links made before linking refused a GitHub account another user is linked to can do that.
	errAmbiguous = errors.New("github: more than one user is linked to the GitHub account")
	// errNoEmail means the GitHub account has no verified primary email to make a new user with.
	errNoEmail = errors.New("github: the GitHub account has no verified primary email")
	// errAccountExists means a user the GitHub account is not linked to has its email. Signing in as them would let
	// whoever holds a GitHub account with that email into theirs, so they sign in with their password and link it.
	errAccountExists = errors.New("github: a user not linked to the GitHub account has its email")
)

// signInCallback finishes signing in with GitHub and sends the browser, signed in, where the sign-in's start said or
// home, or else back to sign-in with the outcome.
func (l *Links) signInCallback(w http.ResponseWriter, r *http.Request) {
	l.stateCookie().Clear(w)
	userID, err := l.finishSignIn(r)
	if err == nil {
		err = l.Accounts.SignIn(r.Context(), w, userID) // a browser signed in already swaps its session cookie for this one
	}
	var outcome string
	switch {
	case err == nil:
		http.Redirect(w, r, l.PublicURL+l.next(r), http.StatusFound)
		return
	case errors.Is(err, errDenied):
		outcome = "denied"
	case errors.Is(err, errNoEmail):
		outcome = "no_email"
	case errors.Is(err, errAccountExists):
		outcome = "account_exists"
	case errors.Is(err, errAmbiguous):
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		outcome = "ambiguous"
	default:
		// Never with the code, the tokens or the client secret.
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		outcome = "error"
	}
	l.login(w, r, outcome)
}

// finishSignIn returns the user the callback signs in as, linked to the GitHub account it authorizes with that
// credential: the one user linked to it already, whose link it replaces, or else a new one, made with the account's
// verified primary email and no password. It saves nothing on failure.
func (l *Links) finishSignIn(r *http.Request) (int64, error) {
	ctx := r.Context()
	u, tok, err := l.authorized(r, oauth.SignIn, "")
	if err != nil {
		return 0, err
	}
	linked, err := linkedTo(ctx, l.DB, u.ID)
	if err != nil {
		return 0, err
	}
	email := ""
	if len(linked) == 0 { // only a new user needs it
		if email, err = l.primaryEmail(ctx, tok.AccessToken); err != nil {
			return 0, err
		}
	}
	var userID int64
	err = pgx.BeginFunc(ctx, l.DB, func(tx pgx.Tx) error {
		// One GitHub account's sign-ins and links take turns, so two cannot both find it unlinked and make two users,
		// and a link cannot make a second.
		if err := lockGitHubAccount(ctx, tx, u.ID); err != nil {
			return err
		}
		linked, err := linkedTo(ctx, tx, u.ID)
		switch {
		case err != nil:
			return err
		case len(linked) > 1:
			return fmt.Errorf("%w: GitHub account %d, %q", errAmbiguous, u.ID, u.Login)
		case len(linked) == 1: // linked before, or by the sign-in this one waited for
			userID = linked[0]
		case email == "":
			return fmt.Errorf("github: GitHub account %d was unlinked while it signed in", u.ID)
		default:
			// email is citext, so another case of it is the same email.
			err := tx.QueryRow(ctx, `insert into users (email) values ($1) on conflict (email) do nothing returning id`,
				email).Scan(&userID)
			if errors.Is(err, pgx.ErrNoRows) {
				return errAccountExists
			}
			if err != nil {
				return err
			}
		}
		return l.save(ctx, tx, userID, u, tok, "oauth")
	})
	return userID, err
}

// linkedTo lists, through q, up to two of the users linked to GitHub account id with OAuth: enough to tell one from
// more. A link made with a personal access token shows only that someone held a token of the account, such as one a
// third-party app was given, not that they can sign in to it, so it signs no one in.
func linkedTo(ctx context.Context, q querier, id int64) ([]int64, error) {
	rows, err := q.Query(ctx, `select user_id from github_links where github_id = $1 and kind = 'oauth' order by user_id limit 2`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// primaryEmail is the primary email of the GitHub account token belongs to, if GitHub verified it, else errNoEmail.
func (l *Links) primaryEmail(ctx context.Context, token string) (string, error) {
	emails, err := l.Client.Emails(ctx, token)
	if err != nil {
		return "", fmt.Errorf("list the GitHub user's emails: %w", err)
	}
	for _, e := range emails {
		if e.Primary && e.Verified && e.Email != "" {
			return e.Email, nil
		}
	}
	return "", errNoEmail
}
