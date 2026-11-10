package planning_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gantt-stop-me/api/internal/llm/llmtest"
)

func TestReviewOffApprovesTheDoc(t *testing.T) {
	f := setup(t)
	f.Do("PUT", "/api/me/planning", agents(f.provider, llmtest.ChatModel, false), f.ada).
		Expect(http.StatusOK, "")
	pl := f.start("Exports", "Let admins export the board as CSV.")
	f.llm.Reply(reply(ask("Who exports?")))
	f.work()
	f.act(pl.ID, "draft", nil).Expect(http.StatusOK, "")
	f.llm.Reply(reply(update(sec("goal", "Export the board.", "covered")), finish))
	f.work()
	if pl = f.plan(pl.ID); pl.Status != "approved" || orNull(pl.Pending) != "generate" || pl.section("goal").State != "covered" {
		t.Fatalf("plan %+v, want it approved without review", pl)
	}
}

func TestWorkRunsOneTurnPerCreator(t *testing.T) {
	f := setup(t)
	a := f.start("Exports", "Export a CSV.")
	b := f.start("Imports", "Import a CSV.")
	f.llm.Reply(reply(ask("Export what?")), reply(ask("Import what?")), reply(ask("Never asked")))
	f.work() // b waits: its creator has a turn running on a
	if a, b = f.plan(a.ID), f.plan(b.ID); len(a.Messages) != 1 || a.Messages[0].Text != "Export what?" ||
		len(b.Messages) != 0 || orNull(b.Pending) != "ask" {
		t.Fatalf("plans after one pass %+v, %+v", a, b)
	}
	f.work()
	if b = f.plan(b.ID); len(b.Messages) != 1 || b.Messages[0].Text != "Import what?" {
		t.Fatalf("plan after the second pass %+v", b)
	}
	f.work()
	if n := len(f.llm.Chats()); n != 2 {
		t.Fatalf("%d chat requests after a pass with nothing pending, want one turn per plan", n)
	}
}

func TestASlowProviderDelaysOnlyItsCreatorsPlans(t *testing.T) {
	f := setup(t)
	// Both providers are the fake, through one server. Bob's, under /slow, answers a chat only once ada's has: were
	// turns run one after another, his plan's, which comes first, would wait for hers until it timed out.
	answered, fake := make(chan struct{}), f.llm.Server.Config.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var slow bool
		r.URL.Path, slow = strings.CutPrefix(r.URL.Path, "/slow")
		chat := r.URL.Path == "/v1/responses"
		if slow && chat {
			select {
			case <-answered:
			case <-r.Context().Done():
				return
			}
		}
		fake.ServeHTTP(w, r)
		if !slow && chat {
			close(answered)
		}
	}))
	t.Cleanup(srv.Close)
	plansWith := func(session *http.Cookie, url string) {
		res := f.Do("POST", "/api/providers", map[string]any{"name": "Gated", "baseUrl": url, "apiKey": f.llm.Key, "models": f.llm.Models}, session)
		res.Expect(http.StatusCreated, "")
		var pv struct{ ID int64 }
		res.JSON(&pv)
		f.Do("PUT", "/api/me/planning", agents(pv.ID, llmtest.ChatModel, true), session).Expect(http.StatusOK, "")
	}
	bob := f.member("bob", "push")
	plansWith(bob, srv.URL+"/slow/v1")
	plansWith(f.ada, srv.URL+"/v1")
	slow := decode(t, f.Do("POST", f.path("/plans"), map[string]string{"title": "Imports", "description": "Import a CSV."}, bob), http.StatusCreated)
	fast := f.start("Exports", "Export a CSV.")

	f.llm.Reply(reply(ask("Export what?")), reply(ask("Import what?")))
	f.work()
	if s, a := f.plan(slow.ID), f.plan(fast.ID); len(a.Messages) != 1 || a.Messages[0].Text != "Export what?" ||
		len(s.Messages) != 1 || s.Messages[0].Text != "Import what?" || s.Error != nil {
		t.Fatalf("plans after one pass %+v, %+v; want ada's turn done while bob's waited", s, a)
	}
}

func TestTurnWithoutAccessOrKey(t *testing.T) {
	const lost = "The plan's creator can no longer see the repository on GitHub. Once they can, someone retries."
	for _, c := range []struct {
		name, change, want string
	}{
		{"lost access", `update project_members set checked_at = now() - interval '5 minutes 1 second'`, lost},
		{"unlinked", `delete from github_links where login = 'ada'`, lost},
		{"unreadable key", `update providers set key_enc = 'garbage'`,
			"The plan's creator's provider key cannot be read. They enter it again in Settings, then someone retries."},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := setup(t)
			reader := f.member("bob", "pull")
			pl := f.start("Exports", "Let admins export the board as CSV.")
			f.repo.Grant(f.token, "") // only counts once the cached permission expires
			if _, err := f.DB.Exec(context.Background(), c.change); err != nil {
				t.Fatal(err)
			}
			f.work()
			if pl = decode(t, f.Do("GET", f.path("/plans/%d", pl.ID), nil, reader), http.StatusOK); pl.Pending != nil || orNull(pl.Error) != c.want {
				t.Fatalf("plan %+v", pl)
			}
			if n := len(f.llm.Chats()); n != 0 {
				t.Fatalf("%d chat requests", n)
			}
		})
	}
}

// relinkExpiring relinks ada with OAuth to a token that GitHub lets push and that expires in 8 hours, and returns it.
func (f *fixture) relinkExpiring() string {
	f.t.Helper()
	f.GitHub.ExpireTokens(8 * time.Hour)
	token := f.LinkGitHubOAuth(f.ada, "ada")
	f.repo.Grant(token, "push")
	return token
}

// refresh is another caller refreshing ada's token as it nears expiry, which GitHub retires as it answers. It returns
// the new token. It may run outside the test's goroutine, so it reports failure as an error.
func (f *fixture) refresh() (string, error) {
	ctx := context.Background()
	var id int64
	if err := f.DB.QueryRow(ctx, `update github_links set expires_at = now() + interval '1 minute'
		where user_id = (select id from users where email = 'ada@example.com') returning user_id`).Scan(&id); err != nil {
		return "", err
	}
	return f.Credentials.Token(ctx, id)
}

// GitHub retires a token as another caller refreshes it, so a 401 for the token the turn read is checked with the
// token read again, and lost access only when GitHub refuses that one too.
func TestTurnAfterAnotherCallerRefreshedTheToken(t *testing.T) {
	const lost = "The plan's creator can no longer see the repository on GitHub. Once they can, someone retries."
	for _, c := range []struct {
		name      string
		rejectNew bool
	}{{"the new token works", false}, {"GitHub refuses the new token too", true}} {
		t.Run(c.name, func(t *testing.T) {
			f := setup(t)
			old := f.relinkExpiring()
			pl := f.start("Exports", "Let admins export the board as CSV.")
			if _, err := f.DB.Exec(context.Background(), `update project_members set checked_at = now() - interval '5 minutes 1 second'`); err != nil {
				t.Fatal(err)
			}
			// While GitHub serves the turn's access check, another caller refreshes the token it was made with.
			var once sync.Once
			var refreshed string
			var err error
			f.GitHub.Before("GET /repositories/{id}", func() {
				once.Do(func() {
					refreshed, err = f.refresh()
					if c.rejectNew {
						f.GitHub.Fail("GET /repositories/{id}", http.StatusUnauthorized)
					}
				})
			})
			f.llm.Reply(reply(ask("Who exports?")))
			f.work()
			if err != nil || refreshed == "" || refreshed == old {
				t.Fatalf("the other caller's refresh: Token = %q, %v; want a new token", refreshed, err)
			}
			f.GitHub.Fail("GET /repositories/{id}", 0)

			pl = f.plan(pl.ID)
			if c.rejectNew {
				if pl.Pending != nil || orNull(pl.Error) != lost || len(f.llm.Chats()) != 0 {
					t.Fatalf("plan %+v after %d chat requests, want lost access and none", pl, len(f.llm.Chats()))
				}
				return
			}
			if pl.Error != nil || len(pl.Messages) != 1 || pl.Messages[0].Text != "Who exports?" {
				t.Fatalf("plan %+v, want the turn run with the refreshed token", pl)
			}
		})
	}
}
