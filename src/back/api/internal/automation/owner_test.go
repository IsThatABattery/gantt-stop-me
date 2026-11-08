package automation_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

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
	f.expectPaused("GitHub no longer accepts ada's credential.")
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
	t.Run("a credential the owner replaced", func(t *testing.T) {
		f := on(t)
		f.LinkGitHub(f.ada, "ada")
		if err := f.automation().PauseIfRejected(ctx, f.project, f.token, rejected); err != rejected || f.pausedReason() != nil {
			t.Fatalf("PauseIfRejected = %v, paused_reason %v; want the error back and no pause", err, f.pausedReason())
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
