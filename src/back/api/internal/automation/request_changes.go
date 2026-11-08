package automation

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

var (
	noTask         = &refused{http.StatusNotFound, "not_found", "Task not found."}
	notInUserCheck = &refused{http.StatusConflict, "not_in_user_check", "Only a task in User Check can have changes requested."}
	noPullRequest  = &refused{http.StatusConflict, "no_pull_request", "This task has no open pull request to comment on."}
)

// requestChanges posts a write member's feedback on a task's linked pull request, with their credential, then moves
// the task back to In Progress and turns the pull request back into a draft, as a failed test run does, so marking it
// ready again starts a new test run. It answers the task as Get task does.
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
	pr, err := a.sendBack(r.Context(), p, token, taskID, feedback)
	if err == nil {
		a.draft(context.WithoutCancel(r.Context()), p, token, pr)
	}
	a.answerTask(w, r, p, token, taskID, err)
}

// sendBack comments feedback, signed with the caller's login, on the open pull request closing task taskID's issue,
// preferring the one its latest test run tested, and moves the task to the end of In Progress, under the project's
// lock. It returns the pull request's number.
func (a *Automation) sendBack(ctx context.Context, p projects.Project, token string, taskID int64, feedback string) (int, error) {
	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return 0, err
	}
	defer unlock()
	// Once GitHub has the comment, finish the move even if the browser goes away.
	ctx = context.WithoutCancel(ctx)
	var number int
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		var issue int
		var column string
		var tested *int
		err := tx.QueryRow(ctx, `
			select t.issue_number, c.key, (select pr_number from test_runs where task_id = t.id order by id desc limit 1)
			from tasks t join columns c on c.id = t.column_id where t.id = $1 and t.project_id = $2`, taskID, p.ID).
			Scan(&issue, &column, &tested)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return noTask
		case err != nil:
			return err
		case column != "user-check":
			return notInUserCheck
		}
		pulls, err := a.GitHub.LinkedPulls(ctx, token, p.RepoID, issue)
		if err != nil {
			return gitHubError{err}
		}
		for _, pr := range pulls {
			if pr.State == "open" && (number == 0 || tested != nil && pr.Number == *tested) {
				number = pr.Number
			}
		}
		if number == 0 {
			return noPullRequest
		}
		var login string
		if err := tx.QueryRow(ctx, `select login from github_links where user_id = $1`, accounts.UserID(ctx)).Scan(&login); err != nil {
			return err
		}
		if err := a.GitHub.CreateComment(ctx, token, p.RepoID, number, "Changes requested by "+login+":\n\n"+feedback); err != nil {
			return gitHubError{err}
		}
		return a.Board.MoveToEnd(ctx, tx, token, p, taskID, "in-progress")
	})
	return number, err
}

// answerTask answers err, or task taskID as Get task does when err is nil.
func (a *Automation) answerTask(w http.ResponseWriter, r *http.Request, p projects.Project, token string, taskID int64, err error) {
	var no *refused
	switch {
	case errors.As(err, &no):
		httpx.Error(w, no.status, no.code, no.message)
	case errors.As(err, new(gitHubError)), errors.As(err, new(*github.Error)): // the latter from moving the task
		github.Fail(w, r, err)
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		a.Board.ServeTask(w, r, p, token, taskID)
	}
}
