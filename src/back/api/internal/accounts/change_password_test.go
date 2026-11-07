package accounts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
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

func TestChangePassword(t *testing.T) {
	env := apptest.New(t)
	current := env.SignUp("ada@example.com", "correct horse")
	other := login(env, "correct horse").Cookie("session")

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
