package github_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"gantt-stop-me/api/internal/apptest"
	"gantt-stop-me/api/internal/github"
)

type storedLink struct {
	GitHubID    int64
	Login, Kind string
	Token       string // credential_enc, decrypted
}

// stored returns every row of github_links, failing the test if a credential is stored in plaintext.
func stored(t *testing.T, env *apptest.Env) []storedLink {
	t.Helper()
	rows, err := env.DB.Query(context.Background(), `select github_id, login, kind, credential_enc from github_links order by user_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var links []storedLink
	for rows.Next() {
		var l storedLink
		var sealed []byte
		if err := rows.Scan(&l.GitHubID, &l.Login, &l.Kind, &sealed); err != nil {
			t.Fatal(err)
		}
		plain, err := env.Config.Secrets.Open(sealed)
		if err != nil {
			t.Fatalf("open credential of %s: %v", l.Login, err)
		}
		if bytes.Contains(sealed, plain) {
			t.Fatalf("credential of %s is stored in plaintext", l.Login)
		}
		l.Token = string(plain)
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return links
}

func userID(t *testing.T, env *apptest.Env, session *http.Cookie) int64 {
	t.Helper()
	var me struct{ ID int64 }
	env.Do("GET", "/api/me", nil, session).JSON(&me)
	return me.ID
}

// addMembership caches access for the session's user to a new project, as opening a board does.
func addMembership(t *testing.T, env *apptest.Env, session *http.Cookie) {
	t.Helper()
	_, err := env.DB.Exec(context.Background(), `
		with p as (
			insert into projects (github_repo_id, owner, name)
			values ((select coalesce(max(github_repo_id), 0) + 1 from projects), 'octo', 'hello') returning id)
		insert into project_members (project_id, user_id, permission, checked_at) select id, $1, 'write', now() from p`,
		userID(t, env, session))
	if err != nil {
		t.Fatal(err)
	}
}

// memberships counts the session user's cached project access.
func memberships(t *testing.T, env *apptest.Env, session *http.Cookie) int {
	t.Helper()
	var n int
	if err := env.DB.QueryRow(context.Background(), `select count(*) from project_members where user_id = $1`, userID(t, env, session)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestToken(t *testing.T) {
	env := apptest.New(t)
	ada := env.SignUp("ada@example.com", "correct horse")
	bob := env.SignUp("bob@example.com", "correct horse")
	token := env.LinkGitHub(ada, "ada-gh")
	ctx := context.Background()

	got, err := github.Token(ctx, env.DB, env.Config.Secrets, userID(t, env, ada))
	if err != nil || got != token {
		t.Fatalf("Token = %q, %v; want the linked token", got, err)
	}
	if _, err := github.Token(ctx, env.DB, env.Config.Secrets, userID(t, env, bob)); !errors.Is(err, github.ErrNotLinked) {
		t.Fatalf("Token for an unlinked user: err = %v, want ErrNotLinked", err)
	}
}
