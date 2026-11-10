package automation

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/board"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

var (
	noTask         = &refused{http.StatusNotFound, "not_found", "Task not found."}
	notInUserCheck = &refused{http.StatusConflict, "not_in_user_check", "Only a task in User Check can have changes requested."}
	noPullRequest  = &refused{http.StatusConflict, "no_pull_request", "This task has no open pull request to comment on."}
)

// requestChanges moves a task back to In Progress, turns its linked pull request back into a draft, as a failed test
// run does, so marking it ready again starts a new test run, and last posts a write member's feedback on it, with
// their credential: a move that fails and is retried then comments once. It answers the task as Get task does.
func (a *Automation) requestChanges(w http.ResponseWriter, r *http.Request) {
	p, token, ok := a.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Feedback string `json:"feedback"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	feedback := strings.TrimSpace(in.Feedback)
	if n := utf8.RuneCountInString(feedback); n < 1 || n > 10000 {
		httpx.Error(w, http.StatusBadRequest, "invalid_feedback", "Feedback needs 1 to 10000 characters.")
		return
	}
	taskID := pathID(r, "taskID")
	pulls, i, login, err := a.sendBack(r.Context(), p, token, taskID)
	if err == nil {
		ctx := context.WithoutCancel(r.Context()) // the task has moved: finish even if the browser goes away
		pr := pulls[i].Number
		drafted := a.draft(ctx, p, token, pr)
		if drafted == nil { // the cache Get task answers from was read before
			pulls[i].Draft = true
			if _, err := a.DB.Exec(ctx, `update tasks set pull_requests = $2 where id = $1`, taskID, pulls); err != nil {
				log.Printf("cache the pull requests of task %d: %v", taskID, err)
			}
		}
		if err = a.GitHub.CreateComment(ctx, token, p.RepoID, pr, "Changes requested by "+login+":\n\n"+feedback); err != nil {
			log.Printf("comment on pull request %d of project %d: %v", pr, p.ID, err)
			err = &refused{http.StatusBadGateway, "github_error",
				"The task is back in In Progress, but GitHub did not take the comment. Post the feedback on the pull request by hand."}
		} else if drafted != nil {
			err = &refused{http.StatusBadGateway, "github_error", "The feedback is posted and the task is back in In Progress, " +
				"but GitHub did not turn the pull request back into a draft. Mark it draft, then ready for review, to test it again."}
		}
	}
	a.answerTask(w, r, p, token, taskID, err)
}

// sendBack moves task taskID to the end of In Progress, under the project's lock, and returns the pull requests
// closing its issue, the index among them of the one to send the feedback to, and the caller's login. That is the
// open one the task's latest test run tested, else an open one ready for review, else an open draft.
func (a *Automation) sendBack(ctx context.Context, p projects.Project, token string, taskID int64) (
	pulls []github.LinkedPull, i int, login string, err error,
) {
	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return nil, 0, "", err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx) // once GitHub has the move, finish saving it
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		var column string
		var tested *int
		err := tx.QueryRow(ctx, `
			select c.key, (select pr_number from test_runs where task_id = t.id order by id desc limit 1)
			from tasks t join columns c on c.id = t.column_id where t.id = $1 and t.project_id = $2`, taskID, p.ID).
			Scan(&column, &tested)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return noTask
		case err != nil:
			return err
		case column != "user-check":
			return notInUserCheck
		}
		pulls, err = a.Board.LinkedPulls(ctx, p, token, taskID) // cached, so the answer reads GitHub no more
		if err != nil {
			return err
		}
		best := 0 // 3 for the tested one, 2 for one ready for review, 1 for a draft
		for j, pr := range pulls {
			rank := 1
			switch {
			case pr.State != "open":
				continue
			case tested != nil && pr.Number == *tested:
				rank = 3
			case !pr.Draft:
				rank = 2
			}
			if rank > best {
				best, i = rank, j
			}
		}
		if best == 0 {
			return noPullRequest
		}
		if err := tx.QueryRow(ctx, `select login from github_links where user_id = $1`, accounts.UserID(ctx)).Scan(&login); err != nil {
			return err
		}
		return a.Board.MoveToEnd(ctx, tx, token, p, taskID, "in-progress")
	})
	return pulls, i, login, err
}

// answerTask answers err, or task taskID as Get task does when err is nil. token is the caller's credential, which
// made the GitHub calls err may come from.
func (a *Automation) answerTask(w http.ResponseWriter, r *http.Request, p projects.Project, token string, taskID int64, err error) {
	var no *refused
	switch {
	case errors.As(err, &no):
		httpx.Error(w, no.status, no.code, no.message)
	case errors.As(err, new(gitHubError)), errors.As(err, new(board.GitHubError)), errors.As(err, new(*github.Error)):
		a.Credentials.Fail(w, r, accounts.UserID(r.Context()), token, err)
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		a.Board.ServeTask(w, r, p, token, taskID)
	}
}
