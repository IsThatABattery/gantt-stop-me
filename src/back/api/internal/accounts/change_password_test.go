package accounts_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gantt-stop-me/api/internal/apptest"
)

func login(env *apptest.Env, password string) *apptest.Response {
	return env.Do("POST", "/api/auth/login", map[string]string{"email": "ada@example.com", "password": password})
}

// send is env.Do for goroutines other than the test's: it returns status 0 on failure instead of failing the test.
func send(env *apptest.Env, method, path string, body any, cookie *http.Cookie) (status int, session *http.Cookie) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, env.Server.URL+path, bytes.NewReader(data))
	req.Header.Set("Origin", apptest.PublicURL)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil
	}
	res.Body.Close()
	for _, c := range res.Cookies() {
		if c.Name == "session" {
			session = c
		}
	}
	return res.StatusCode, session
}

// age makes session as old as d, as though it started d ago.
func age(t *testing.T, env *apptest.Env, session *http.Cookie, d time.Duration) {
	t.Helper()
	sum := sha256.Sum256([]byte(session.Value))
	tag, err := env.DB.Exec(context.Background(), `update sessions set created_at = now() - make_interval(secs => $2) where id = $1`,
		hex.EncodeToString(sum[:]), d.Seconds())
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("age the session: %v, %d rows", err, tag.RowsAffected())
	}
}

func TestChangePassword(t *testing.T) {
	env := apptest.New(t)
	current := env.SignUp("ada@example.com", "correct horse")
	other := login(env, "correct horse").Cookie("session")
	age(t, env, current, 24*time.Hour) // the current password is the proof, so a session of any age changes it

	env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "correct horse", "newPassword": "battery staple"}, current).
		Expect(http.StatusNoContent, "")

	login(env, "correct horse").Expect(http.StatusUnauthorized, "invalid_credentials")
	login(env, "battery staple").Expect(http.StatusOK, "")
	env.Do("GET", "/api/me", nil, current).Expect(http.StatusOK, "")
	env.Do("GET", "/api/me", nil, other).Expect(http.StatusUnauthorized, "unauthorized")
}

func TestChangePasswordChecksCurrentPassword(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "wrong horse", "newPassword": "battery staple"}, session).
		Expect(http.StatusForbidden, "wrong_password")
	login(env, "correct horse").Expect(http.StatusOK, "")
}

func TestChangePasswordValidatesInput(t *testing.T) {
	env := apptest.New(t)
	session := env.SignUp("ada@example.com", "correct horse")

	env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "correct horse", "newPassword": "short"}, session).
		Expect(http.StatusBadRequest, "weak_password")
	env.Do("PATCH", "/api/me/password", "{", session).Expect(http.StatusBadRequest, "bad_request")
	env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "correct horse", "newPassword": "battery staple"}).
		Expect(http.StatusUnauthorized, "unauthorized")
}

// Two sessions change the password at once. The second change must fail, not silently sign both out.
func TestChangePasswordConcurrent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		newPw  [2]string
		second int // status of the change that runs second
	}{
		{"different passwords", [2]string{"battery staple", "stolen password"}, http.StatusForbidden},
		// The current password still matches, so only the session check stops the second change.
		{"same password", [2]string{"correct horse", "correct horse"}, http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := apptest.New(t)
			sessions := []*http.Cookie{env.SignUp("ada@example.com", "correct horse"), login(env, "correct horse").Cookie("session")}
			ctx := context.Background()
			// Hold ada's row lock so both changes are in flight before either can write.
			tx, err := env.DB.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, `select 1 from users for update`); err != nil {
				t.Fatal(err)
			}
			var statuses [2]int
			var wg sync.WaitGroup
			for i, c := range sessions {
				wg.Go(func() {
					statuses[i], _ = send(env, "PATCH", "/api/me/password",
						map[string]string{"currentPassword": "correct horse", "newPassword": tc.newPw[i]}, c)
				})
			}
			for waiting, deadline := 0, time.Now().Add(5*time.Second); waiting < 2; {
				if time.Now().After(deadline) {
					t.Fatalf("%d of 2 changes reached the row lock", waiting)
				}
				time.Sleep(10 * time.Millisecond)
				// Row-lock waits only: other packages' tests wait on the migrations' advisory lock.
				err := env.DB.QueryRow(ctx, `select count(*) from pg_stat_activity
					where datname = current_database() and wait_event in ('transactionid', 'tuple')`).Scan(&waiting)
				if err != nil {
					t.Fatal(err)
				}
			}
			tx.Rollback(ctx)
			wg.Wait()
			slices.Sort(statuses[:])
			if want := [2]int{http.StatusNoContent, tc.second}; statuses != want {
				t.Fatalf("concurrent changes got %v, want %v", statuses, want)
			}
		})
	}
}

// A login that checked the old password while the change committed must not keep a session.
func TestChangePasswordEndsLoginsInFlight(t *testing.T) {
	env := apptest.New(t)
	current := env.SignUp("ada@example.com", "correct horse")
	var (
		mu       sync.Mutex
		sessions []*http.Cookie
		stop     atomic.Bool
		wg       sync.WaitGroup
	)
	for range 3 {
		wg.Go(func() {
			for !stop.Load() {
				status, c := send(env, "POST", "/api/auth/login", map[string]string{"email": "ada@example.com", "password": "correct horse"}, nil)
				if status != http.StatusOK {
					return
				}
				mu.Lock()
				sessions = append(sessions, c)
				mu.Unlock()
			}
		})
	}
	time.Sleep(300 * time.Millisecond)
	env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "correct horse", "newPassword": "battery staple"}, current).
		Expect(http.StatusNoContent, "")
	stop.Store(true)
	wg.Wait()
	for _, c := range sessions {
		env.Do("GET", "/api/me", nil, c).Expect(http.StatusUnauthorized, "unauthorized")
	}
}

// An account made by signing in with GitHub sets its first password with no current one, from a fresh session,
// and that ends its other sessions, as changing a password does.
func TestSetFirstPassword(t *testing.T) {
	env := apptest.New(t)
	for _, body := range []map[string]string{
		{"newPassword": "battery staple"},
		{"currentPassword": "anything at all", "newPassword": "battery staple"}, // ignored
	} {
		name := strings.ToLower(rand.Text()) // a new GitHub account, and user, each time
		other, _ := env.SignInWithGitHub(name, name+"@example.com")
		current, _ := env.SignInWithGitHub(name, name+"@example.com")
		age(t, env, current, freshFor-time.Minute)

		env.Do("PATCH", "/api/me/password", map[string]string{"newPassword": "short"}, current).Expect(http.StatusBadRequest, "weak_password")
		env.Do("PATCH", "/api/me/password", body, current).Expect(http.StatusNoContent, "")
		env.Do("GET", "/api/me", nil, current).Expect(http.StatusOK, "")
		env.Do("GET", "/api/me", nil, other).Expect(http.StatusUnauthorized, "unauthorized")
		env.Do("POST", "/api/auth/login", map[string]string{"email": name + "@example.com", "password": "battery staple"}).
			Expect(http.StatusOK, "")

		// Now there is one, so changing it is as for any account: it needs the current one, from a session of any age.
		age(t, env, current, 24*time.Hour)
		env.Do("PATCH", "/api/me/password", map[string]string{"newPassword": "stolen password"}, current).
			Expect(http.StatusForbidden, "wrong_password")
		env.Do("PATCH", "/api/me/password", map[string]string{"currentPassword": "battery staple", "newPassword": "correct horse"}, current).
			Expect(http.StatusNoContent, "")
		env.Do("POST", "/api/auth/login", map[string]string{"email": name + "@example.com", "password": "correct horse"}).
			Expect(http.StatusOK, "")
	}
}

// freshFor is how new a session must be to set a first password.
const freshFor = 10 * time.Minute

// An account with no password signs in only with GitHub, so only a session it started moments ago shows its user
// holds the GitHub account: an older one may be stolen, and setting a password with it would let the thief unlink
// GitHub and lock the user out.
func TestSetFirstPasswordNeedsAFreshSession(t *testing.T) {
	env := apptest.New(t)
	stale, _ := env.SignInWithGitHub("octocat", "octo@example.com")
	age(t, env, stale, freshFor+time.Minute)
	other, _ := env.SignInWithGitHub("octocat", "octo@example.com")

	res := env.Do("PATCH", "/api/me/password", map[string]string{"newPassword": "battery staple"}, stale)
	res.Expect(http.StatusForbidden, "reauth_required")
	var body struct{ Error struct{ Message string } }
	res.JSON(&body)
	if want := "Sign in with GitHub again, then set your password."; body.Error.Message != want {
		t.Fatalf("message = %q, want %q", body.Error.Message, want)
	}
	env.Do("POST", "/api/auth/login", map[string]string{"email": "octo@example.com", "password": "battery staple"}).
		Expect(http.StatusUnauthorized, "invalid_credentials")
	env.Do("GET", "/api/me", nil, other).Expect(http.StatusOK, "")
	var me struct{ HasPassword bool }
	env.Do("GET", "/api/me", nil, stale).JSON(&me)
	if me.HasPassword {
		t.Fatal("hasPassword = true after the refused change")
	}

	// Signing in with GitHub again is the fresh session that sets it.
	env.Do("PATCH", "/api/me/password", map[string]string{"newPassword": "battery staple"}, other).Expect(http.StatusNoContent, "")
	env.Do("GET", "/api/me", nil, stale).Expect(http.StatusUnauthorized, "unauthorized")
}
