package automation_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"gantt-stop-me/api/internal/automation"
	"gantt-stop-me/api/internal/github"
)

// expire makes every cached permission older than five minutes, so the next check asks GitHub.
func (f *fixture) expire() {
	f.t.Helper()
	f.exec(`update project_members set checked_at = now() - interval '5 minutes 1 second'`)
}

// cached is ada's cached permission on the project, or "" for none.
func (f *fixture) cached() string {
	f.t.Helper()
	var perm string
	f.DB.QueryRow(context.Background(), `select permission from project_members where project_id = $1 and user_id = $2`, f.project, f.adaID).Scan(&perm)
	return perm
}

// on is a fixture with automation on and ada its owner.
func on(t *testing.T) *fixture {
	f := setup(t)
	f.enable(f.ada, f.provider).Expect(http.StatusOK, "")
	return f
}

// expectPaused checks OwnerToken answers ErrPaused and the project is paused with this reason.
func (f *fixture) expectPaused(reason string) {
	f.t.Helper()
	if _, err := f.automation().OwnerToken(context.Background(), f.project); !errors.Is(err, automation.ErrPaused) {
		f.t.Fatalf("OwnerToken err = %v, want ErrPaused", err)
	}
	if got := f.pausedReason(); got == nil || *got != reason {
		f.t.Fatalf("paused_reason = %v, want %q", got, reason)
	}
}

func TestOwnerToken(t *testing.T) {
	f := on(t)
	a := f.automation()
	before := len(f.GitHub.Requests())
	token, err := a.OwnerToken(context.Background(), f.project)
	if err != nil || token != f.token {
		t.Fatalf("OwnerToken = %q, %v, want ada's token", token, err)
	}
	if got := f.GitHub.Requests()[before:]; len(got) != 0 {
		t.Fatalf("a fresh cached permission asked GitHub: %v", got)
	}

	f.expire()
	before = len(f.GitHub.Requests())
	if token, err := a.OwnerToken(context.Background(), f.project); err != nil || token != f.token {
		t.Fatalf("OwnerToken after the cache expired = %q, %v", token, err)
	}
	if got := f.GitHub.Requests()[before:]; len(got) != 1 || got[0] != "GET /repositories/1" {
		t.Fatalf("GitHub requests = %v, want one repository read", got)
	}
	if f.pausedReason() != nil {
		t.Fatal("paused a project whose owner can push")
	}
}

// linkExpiring relinks ada with OAuth to a token that expires in a minute, and returns it.
func (f *fixture) linkExpiring() string {
	f.t.Helper()
	f.GitHub.ExpireTokens(8 * time.Hour)
	token := f.LinkGitHubOAuth(f.ada, "ada")
	f.repo.Grant(token, "push")
	f.exec(`update github_links set expires_at = now() + interval '1 minute' where user_id = $1`, f.adaID)
	return token
}

func TestOwnerTokenRefreshesAnExpiringToken(t *testing.T) {
	f := on(t)
	old := f.linkExpiring()
	token, err := f.automation().OwnerToken(context.Background(), f.project)
	if err != nil || token == old || token == "" || f.GitHub.Refreshes() != 1 {
		t.Fatalf("OwnerToken = %q, %v after %d refreshes; want a new token from one refresh", token, err, f.GitHub.Refreshes())
	}
	if f.pausedReason() != nil {
		t.Fatal("paused a project whose owner's token was refreshed")
	}
}

func TestOwnerTokenPausesWhenOwnerLosesWriteAccess(t *testing.T) {
	for _, role := range []string{"pull", "none", ""} { // "" hides the repository from ada
		t.Run(role, func(t *testing.T) {
			f := on(t)
			f.repo.Grant(f.token, role)
			f.expire()
			f.expectPaused("ada lost write access to the repository.")
			want := map[string]string{"pull": "read"}[role]
			if got := f.cached(); got != want {
				t.Fatalf("cached permission = %q, want %q", got, want)
			}
		})
	}
}

func TestOwnerTokenPausesOnCachedReadAccess(t *testing.T) {
	f := on(t)
	f.exec(`update project_members set permission = 'read'`) // as a fresh check by a project route would leave it
	before := len(f.GitHub.Requests())
	f.expectPaused("ada lost write access to the repository.")
	if got := f.GitHub.Requests()[before:]; len(got) != 0 {
		t.Fatalf("a fresh cached permission asked GitHub: %v", got)
	}
}

func TestOwnerTokenPausesWhenOwnerUnlinks(t *testing.T) {
	f := on(t)
	f.Do("DELETE", "/api/github", nil, f.ada).Expect(http.StatusNoContent, "")
	f.expectPaused("The automation owner unlinked their GitHub account.")
}

func TestOwnerTokenPausesWhenGitHubRejectsTheCredential(t *testing.T) {
	f := on(t)
	f.expire()
	f.GitHub.Fail("GET /repositories/{id}", http.StatusUnauthorized)
	before := len(f.GitHub.Requests())
	f.expectPaused("GitHub no longer accepts ada's credential.")
	// The credential read again is the one GitHub refused, so it is not tried again.
	if got := f.GitHub.Requests()[before:]; len(got) != 1 || got[0] != "GET /repositories/1" {
		t.Fatalf("GitHub requests = %v, want one repository read", got)
	}
}

// refreshDuring has another caller refresh ada's expiring token while GitHub serves OwnerToken's first repository
// read, so GitHub refuses the token that read was made with. With rejectNew, GitHub refuses the new token too. It
// returns ada's token from before and a function that returns the new one.
func (f *fixture) refreshDuring(rejectNew bool) (old string, refreshed func() string) {
	f.t.Helper()
	old = f.linkExpiring()
	f.exec(`update github_links set expires_at = now() + interval '1 hour' where user_id = $1`, f.adaID) // read unrefreshed
	f.expire()
	var once sync.Once
	var token string
	var err error
	f.GitHub.Before("GET /repositories/{id}", func() {
		once.Do(func() {
			ctx := context.Background()
			if _, err = f.DB.Exec(ctx, `update github_links set expires_at = now() + interval '1 minute' where user_id = $1`, f.adaID); err == nil {
				token, err = f.Credentials.Token(ctx, f.adaID)
			}
			if rejectNew {
				f.GitHub.Fail("GET /repositories/{id}", http.StatusUnauthorized)
			}
		})
	})
	return old, func() string {
		f.t.Helper()
		if err != nil || token == "" || token == old {
			f.t.Fatalf("the other caller's refresh: Token = %q, %v; want a new token", token, err)
		}
		return token
	}
}

func TestOwnerTokenAfterAnotherCallerRefreshedTheToken(t *testing.T) {
	f := on(t)
	_, refreshed := f.refreshDuring(false)
	before := len(f.GitHub.Requests())
	token, err := f.automation().OwnerToken(context.Background(), f.project)
	if err != nil || token != refreshed() {
		t.Fatalf("OwnerToken = %q, %v; want the token the other caller refreshed", token, err)
	}
	if f.pausedReason() != nil || f.cached() != "write" {
		t.Fatalf("paused_reason %v, cached permission %q; want automation running and the check made with the new token", f.pausedReason(), f.cached())
	}
	want := []string{"POST /login/oauth/access_token", "GET /repositories/1", "GET /repositories/1"}
	if got := f.GitHub.Requests()[before:]; !slices.Equal(got, want) {
		t.Fatalf("GitHub requests = %v, want the other caller's refresh, the repository read it beat, and the read again", got)
	}
}

func TestOwnerTokenPausesWhenGitHubRejectsTheRefreshedTokenToo(t *testing.T) {
	f := on(t)
	_, refreshed := f.refreshDuring(true)
	f.expectPaused("GitHub no longer accepts ada's credential.")
	refreshed()
}

func TestOwnerTokenTemporaryErrorsDoNotPause(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			f := on(t)
			f.expire()
			f.GitHub.Fail("GET /repositories/{id}", status)
			_, err := f.automation().OwnerToken(context.Background(), f.project)
			var ghErr *github.Error
			if !errors.As(err, &ghErr) || ghErr.Status != status {
				t.Fatalf("OwnerToken err = %v, want GitHub's %d", err, status)
			}
			if f.pausedReason() != nil || f.cached() != "write" {
				t.Fatalf("a temporary error paused the project or dropped the cached permission %q", f.cached())
			}
			f.GitHub.Fail("GET /repositories/{id}", 0)
			if token, err := f.automation().OwnerToken(context.Background(), f.project); err != nil || token != f.token {
				t.Fatalf("OwnerToken once GitHub recovered = %q, %v", token, err)
			}
		})
	}
}

func TestOwnerTokenWhilePaused(t *testing.T) {
	f := on(t)
	f.exec(`update project_automation set paused_reason = 'GitHub no longer accepts ada''s credential.'`)
	f.expire()
	before := len(f.GitHub.Requests())
	f.expectPaused("GitHub no longer accepts ada's credential.") // even though ada can push again
	if got := f.GitHub.Requests()[before:]; len(got) != 0 {
		t.Fatalf("a paused project asked GitHub: %v", got)
	}
}

func TestOwnerTokenWhenOff(t *testing.T) {
	f := setup(t)
	if _, err := f.automation().OwnerToken(context.Background(), f.project); !errors.Is(err, automation.ErrOff) {
		t.Fatalf("OwnerToken err = %v, want ErrOff", err)
	}
}

func TestTakeoverUnpauses(t *testing.T) {
	f := on(t)
	f.repo.Grant(f.token, "pull")
	f.expire()
	f.expectPaused("ada lost write access to the repository.")

	carol, carolID, carolToken := f.member("carol", "push")
	f.enable(carol, f.addProvider(carolID, "luna", "chat")).Expect(http.StatusOK, "")
	if token, err := f.automation().OwnerToken(context.Background(), f.project); err != nil || token != carolToken {
		t.Fatalf("OwnerToken after carol took over = %q, %v, want carol's token", token, err)
	}
}

func TestOwnerTokenChangeDuringTheCheck(t *testing.T) {
	// during runs SQL while GitHub checks ada, then has GitHub reject her credential.
	during := func(f *fixture, sql string, args ...any) *automation.Automation {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := f.DB.Exec(r.Context(), sql, args...); err != nil {
				f.t.Error(err)
			}
			w.WriteHeader(http.StatusUnauthorized)
		}))
		f.t.Cleanup(srv.Close)
		a := f.automation()
		a.Projects.GitHub = &github.Client{API: srv.URL, HTTP: srv.Client()}
		f.expire()
		return a
	}
	t.Run("taken over", func(t *testing.T) {
		f := on(t)
		_, carolID, carolToken := f.member("carol", "push")
		a := during(f, `update project_automation set owner_id = $1`, carolID)
		_, err := a.OwnerToken(context.Background(), f.project)
		if err == nil || errors.Is(err, automation.ErrPaused) || errors.Is(err, automation.ErrOff) || f.pausedReason() != nil {
			t.Fatalf("OwnerToken err = %v, paused_reason %v; want a temporary error and carol's automation running", err, f.pausedReason())
		}
		if token, err := f.automation().OwnerToken(context.Background(), f.project); err != nil || token != carolToken {
			t.Fatalf("next OwnerToken = %q, %v, want carol's token", token, err)
		}
	})
	t.Run("turned off", func(t *testing.T) {
		f := on(t)
		a := during(f, `delete from project_automation`)
		if _, err := a.OwnerToken(context.Background(), f.project); !errors.Is(err, automation.ErrOff) {
			t.Fatalf("OwnerToken err = %v, want ErrOff", err)
		}
	})
}

func TestPauseIfRejected(t *testing.T) {
	ctx := context.Background()
	rejected := &github.Error{Status: http.StatusUnauthorized}
	t.Run("other errors", func(t *testing.T) {
		f := on(t)
		a := f.automation()
		for _, err := range []error{nil, &github.Error{Status: http.StatusForbidden}, &github.Error{Status: http.StatusBadGateway}, errors.New("dial")} {
			if got := a.PauseIfRejected(ctx, f.project, f.token, err); got != err {
				t.Fatalf("PauseIfRejected(%v) = %v", err, got)
			}
		}
		if f.pausedReason() != nil {
			t.Fatal("paused on an error that is not a 401")
		}
	})
	t.Run("the owner's credential", func(t *testing.T) {
		f := on(t)
		if err := f.automation().PauseIfRejected(ctx, f.project, f.token, rejected); !errors.Is(err, automation.ErrPaused) {
			t.Fatalf("PauseIfRejected = %v, want ErrPaused", err)
		}
		f.expectPaused("GitHub no longer accepts ada's credential.")
	})
	t.Run("automation off", func(t *testing.T) {
		f := setup(t)
		if err := f.automation().PauseIfRejected(ctx, f.project, f.token, rejected); !errors.Is(err, automation.ErrOff) {
			t.Fatalf("PauseIfRejected = %v, want ErrOff", err)
		}
	})
	t.Run("a credential the owner replaced", func(t *testing.T) {
		f := on(t)
		f.LinkGitHub(f.ada, "ada")
		if err := f.automation().PauseIfRejected(ctx, f.project, f.token, rejected); err != rejected || f.pausedReason() != nil {
			t.Fatalf("PauseIfRejected = %v, paused_reason %v; want the error back and no pause", err, f.pausedReason())
		}
	})
	t.Run("a credential refreshed since", func(t *testing.T) {
		f := on(t)
		old := f.linkExpiring()
		if _, err := f.automation().OwnerToken(ctx, f.project); err != nil {
			t.Fatal(err)
		}
		if err := f.automation().PauseIfRejected(ctx, f.project, old, rejected); err != rejected || f.pausedReason() != nil {
			t.Fatalf("PauseIfRejected = %v, paused_reason %v; want the error back and no pause", err, f.pausedReason())
		}
	})
	t.Run("a credential GitHub refused for expiring", func(t *testing.T) {
		f := on(t)
		token := f.linkExpiring()
		// Looking at the owner's credential refreshes it, so the next execution runs with the new one.
		if err := f.automation().PauseIfRejected(ctx, f.project, token, rejected); err != rejected || f.pausedReason() != nil {
			t.Fatalf("PauseIfRejected = %v, paused_reason %v; want the error back and no pause", err, f.pausedReason())
		}
		if n := f.GitHub.Refreshes(); n != 1 {
			t.Fatalf("%d refreshes, want 1", n)
		}
	})
	t.Run("a credential of the owner before a takeover", func(t *testing.T) {
		f := on(t)
		carol, carolID, _ := f.member("carol", "push")
		f.enable(carol, f.addProvider(carolID, "luna", "chat")).Expect(http.StatusOK, "")
		if err := f.automation().PauseIfRejected(ctx, f.project, f.token, rejected); err != rejected || f.pausedReason() != nil {
			t.Fatalf("PauseIfRejected = %v, paused_reason %v; want the error back and no pause", err, f.pausedReason())
		}
	})
}
