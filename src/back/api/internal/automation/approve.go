package automation

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// refused is an approval answered with status and an error code and message instead of a merge.
type refused struct {
	status        int
	code, message string
}

func (r *refused) Error() string { return "automation: approval refused: " + r.message }

var (
	// untested refuses approving a task that is not in User Check with a latest test run that passed.
	untested = &refused{http.StatusConflict, "not_tested", "Only a task in User Check whose latest test run passed can be approved."}
	// siblingUntested refuses approving a task while another task linked to the same pull request is still being tested.
	siblingUntested = &refused{http.StatusConflict, "run_in_progress",
		"Another task linked to this pull request has a test run in progress. Approve once it ends."}
	// closedPull refuses approving a task whose pull request is closed or merged, maybe by an approval just before.
	closedPull = &refused{http.StatusConflict, "pull_request_closed", "The linked pull request is closed or merged already."}
	// takenOverRefusal refuses approving a task a member took over: automation leaves it to them.
	takenOverRefusal = &refused{http.StatusConflict, "taken_over",
		"A member took this task over from automation, so it does not merge it. Give it back to approve it."}
	// headMoved answers an approval that found new commits on the pull request and started a test run of them instead.
	headMoved = &refused{http.StatusConflict, "head_moved",
		"The pull request has new commits since it was tested. The task is back in Testing for a test run of them."}
)

// gitHubError marks a failed GitHub call apart from a database error, so approve answers it with github.Fail.
type gitHubError struct{ error }

func (e gitHubError) Unwrap() error { return e.error }

// approve merges a task's linked pull request once a write member has checked its manual tests by hand, then moves
// the task to Closed, as Approve and merge in docs/api/automation.md says, and answers the approved test run.
func (a *Automation) approve(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	var in struct {
		Checked []int64 `json:"checked"` // ids of the manual test results the approver checked
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	approved, err := a.merge(r.Context(), p, pathID(r, "taskID"), in.Checked)
	var no *refused
	switch {
	case errors.As(err, &no):
		httpx.Error(w, no.status, no.code, no.message)
	case errors.Is(err, ErrOff):
		httpx.Error(w, http.StatusConflict, "automation_off", "Turn automation on first.")
	case errors.Is(err, ErrPaused):
		httpx.Error(w, http.StatusConflict, "automation_paused",
			"Automation is paused, so nothing merges. A write member can take over as its owner to unpause it.")
	// *github.Error from moving the task, and ErrUnavailable for the owner's OAuth token that GitHub could not refresh.
	case errors.As(err, new(gitHubError)), errors.As(err, new(*github.Error)), errors.Is(err, github.ErrUnavailable):
		github.Fail(w, r, err)
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		httpx.JSON(w, http.StatusOK, approved)
	}
}

// merge approves the project's task taskID with checked, the manual test results the caller ticked, merges its pull
// request with the automation owner's credential, and returns the approved test run. It reads GitHub before it takes
// the project's lock, then checks automation and the task again under the lock, so of two approvals at once only the
// first merges. A pull request whose head moved since the run, seen before merging or by GitHub refusing the merge,
// moves the task back to Testing as an ordinary move, with a new test run of the new head, and returns headMoved.
func (a *Automation) merge(ctx context.Context, p projects.Project, taskID int64, checked []int64) (approved run, err error) {
	ap, err := approvable(ctx, a.DB, p.ID, taskID, checked)
	if err != nil {
		return run{}, err
	}
	token, err := a.OwnerToken(ctx, p.ID)
	if err != nil {
		return run{}, err
	}
	defer func() {
		// Every GitHub call below uses token. A 401 that PauseIfRejected pauses nothing for is for a token the owner's
		// credential has since replaced, as another caller's refresh does, or an earlier owner's: no verdict on the
		// approver's credential, so it answers as a failed call, and trying again uses the credential in use now.
		if err = a.PauseIfRejected(ctx, p.ID, token, err); rejected(err) {
			// Not %w: github.Fail answers an error without GitHub's status with 502.
			err = gitHubError{fmt.Errorf("automation: GitHub refused the owner's token, which is no longer the one in use: %v", err)}
		}
	}()
	pr, err := a.GitHub.PullRequest(ctx, token, p.RepoID, ap.pr)
	if err != nil {
		return run{}, gitHubError{err}
	}
	if pr.State != "open" {
		return run{}, closedPull
	}
	method, why := "", "" // why, once the head moved, is why its test run may not run
	if pr.Head.SHA == ap.head {
		if err := unmergeable(pr); err != nil {
			return run{}, err
		}
		method, err = a.GitHub.MergeMethod(ctx, token, pr.NodeID)
	} else {
		why, err = a.retestRefusal(ctx, token, p, pr)
	}
	if err != nil {
		return run{}, gitHubError{err}
	}

	unlock, err := projects.Serialize(ctx, p.ID)
	if err != nil {
		return run{}, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx) // once GitHub has merged, finish saving it
	moved := false
	err = pgx.BeginFunc(ctx, a.DB, func(tx pgx.Tx) error {
		if err := projects.Lock(ctx, tx, p.ID); err != nil {
			return err
		}
		// Turned off or paused since OwnerToken: no merge, and no run that turning it off would have cancelled.
		var paused bool
		err := tx.QueryRow(ctx, `select paused_reason is not null from project_automation where project_id = $1`, p.ID).Scan(&paused)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrOff
		case err != nil:
			return err
		case paused:
			return ErrPaused
		}
		again, err := approvable(ctx, tx, p.ID, taskID, checked)
		if err != nil {
			return err
		}
		if again != ap { // a newer run passed meanwhile, which the approver has not checked
			return untested
		}
		if pr.Head.SHA == ap.head {
			mergeErr := a.GitHub.Merge(ctx, token, p.RepoID, pr.Number, ap.head, method)
			var ghErr *github.Error
			errors.As(mergeErr, &ghErr)
			switch {
			case mergeErr == nil:
				approved, err = a.approved(ctx, tx, p, token, taskID, ap.run)
				return err
			case ghErr != nil && ghErr.Status == http.StatusMethodNotAllowed: // a conflict, a required check, branch protection
				return &refused{http.StatusConflict, "not_mergeable", cmp.Or(ghErr.Message, "GitHub cannot merge this pull request.")}
			case ghErr == nil || ghErr.Status != http.StatusConflict:
				// GitHub may have merged with its answer lost, as on a timeout: only a new read tells.
				if pr, err := a.GitHub.PullRequest(ctx, token, p.RepoID, pr.Number); err == nil && pr.Merged && pr.Head.SHA == ap.head {
					approved, err = a.approved(ctx, tx, p, token, taskID, ap.run)
					return err
				}
				return gitHubError{mergeErr}
			}
			// GitHub's 409: the head moved at the last moment, to a commit only a new read tells.
			if pr, err = a.GitHub.PullRequest(ctx, token, p.RepoID, pr.Number); err != nil {
				return gitHubError{err}
			}
			switch {
			case pr.State != "open":
				return closedPull
			case pr.Head.SHA == ap.head: // the read lags the push, or GitHub refused for another reason: try again
				return gitHubError{mergeErr}
			}
			if why, err = a.retestRefusal(ctx, token, p, pr); err != nil {
				return gitHubError{err}
			}
		}
		if err := a.Board.MoveToEnd(ctx, tx, token, p, taskID, "testing"); err != nil {
			return err
		}
		moved = true
		return startRun(ctx, tx, p.ID, taskID, pr.Number, pr.Head.SHA, why)
	})
	if err == nil && moved {
		err = headMoved
	}
	return approved, err
}

// unmergeable refuses merging pr unless GitHub's mergeable_state says it may merge as it is. The owner's credential
// may be one GitHub lets bypass branch protection, as an admin's, and the merge endpoint then bypasses it without a
// word: an approver with only write access must not borrow that.
func unmergeable(pr github.PullRequest) error {
	why := "GitHub cannot merge this pull request as it is."
	switch pr.MergeableState {
	case "clean", "has_hooks", "unstable": // unstable: only checks that are not required failed
		return nil
	case "blocked":
		why = "Branch protection blocks merging this pull request, such as for a required review or status check."
	case "behind":
		why = "Branch protection wants the pull request's branch up to date with its base. Updating it tests it again."
	case "dirty":
		why = "The pull request has merge conflicts."
	case "draft":
		why = "The pull request is a draft."
	case "unknown":
		why = "GitHub has not worked out yet whether this pull request can merge. Try again in a moment."
	}
	return &refused{http.StatusConflict, "not_mergeable", why}
}

// retestRefusal says why a test run of pr's new head may not run, as the triggers would refuse it, or is "" when it
// may: since the run that passed, the author may have lost write access, or pr may be a draft again.
func (a *Automation) retestRefusal(ctx context.Context, token string, p projects.Project, pr github.PullRequest) (string, error) {
	if pr.Draft { // the draft-to-ready trigger tests it once it is ready
		return "The pull request is a draft. Mark it ready for review to test it.", nil
	}
	return a.refusal(ctx, token, p, pr)
}

// approved saves the caller's approval of test run runID, whose pull request just merged, moves its task to Closed
// with token, and returns the run. Call it in tx holding projects.Lock. The move goes in a savepoint and a failed
// one is only logged: the merge stands whatever the move does, and the issue sync moves the task once GitHub closes
// its issue.
func (a *Automation) approved(ctx context.Context, tx pgx.Tx, p projects.Project, token string, taskID, runID int64) (run, error) {
	if _, err := tx.Exec(ctx, `update test_runs set approved_by = $2, approved_at = now() where id = $1`, runID, accounts.UserID(ctx)); err != nil {
		return run{}, err
	}
	if err := pgx.BeginFunc(ctx, tx, func(sp pgx.Tx) error { return a.Board.MoveToEnd(ctx, sp, token, p, taskID, "closed") }); err != nil {
		log.Printf("move task %d to Closed after merging its pull request: %v", taskID, err)
	}
	rows, _ := tx.Query(ctx, `select `+runColumns+` from test_runs r where r.id = $1`, runID)
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[run])
}

// approval is the test run an approval merges.
type approval struct {
	run  int64
	pr   int    // its pull request's number
	head string // the commit it tested
}

// approvable returns what approving the project's task taskID with checked merges, reading through db, the pool or a
// transaction holding the project's lock. The task must be in User Check with a latest test run that passed on a
// pull request and is not approved yet, and checked must hold exactly that run's manual test results, as a set. No
// other task's test run on that pull request may be queued or running: the merge closes their issues too, and a
// failure among them turns the pull request back into a draft.
func approvable(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID, taskID int64, checked []int64) (approval, error) {
	var ap approval
	var column, status string
	var approved, siblingRunning, takenOver bool
	var ids []int64
	var texts []string
	err := db.QueryRow(ctx, `
		select c.key, t.taken_over_by is not null, coalesce(r.id, 0), coalesce(r.pr_number, 0), coalesce(r.head_sha, ''), coalesce(r.status, ''),
			r.approved_at is not null,
			exists (select from test_runs o join tasks ot on ot.id = o.task_id
				where ot.project_id = t.project_id and o.task_id <> t.id and o.pr_number = r.pr_number
					and o.status in ('queued', 'running')),
			array(select x.id from test_results x where x.run_id = r.id and x.status = 'manual' order by x.position),
			array(select x.text from test_results x where x.run_id = r.id and x.status = 'manual' order by x.position)
		from tasks t join columns c on c.id = t.column_id
		left join lateral (select * from test_runs where task_id = t.id order by id desc limit 1) r on true
		where t.id = $1 and t.project_id = $2`, taskID, projectID).
		Scan(&column, &takenOver, &ap.run, &ap.pr, &ap.head, &status, &approved, &siblingRunning, &ids, &texts)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return approval{}, &refused{http.StatusNotFound, "not_found", "Task not found."}
	case err != nil:
		return approval{}, err
	case takenOver: // the member who took it over merges it, not automation with its owner's credential
		return approval{}, takenOverRefusal
	case approved: // merged by an approval, maybe one at the same time as this
		return approval{}, closedPull
	case column != "user-check" || status != "passed" || ap.pr == 0:
		return approval{}, untested
	case siblingRunning:
		return approval{}, siblingUntested
	}
	var missing, extra []string
	for i, id := range ids {
		if !slices.Contains(checked, id) {
			missing = append(missing, `"`+oneLine(texts[i])+`"`)
		}
	}
	for _, id := range checked {
		if !slices.Contains(ids, id) {
			extra = append(extra, strconv.FormatInt(id, 10))
		}
	}
	if missing == nil && extra == nil {
		return ap, nil
	}
	message := "Check each manual test of the latest test run by hand and tick exactly those before approving."
	if missing != nil {
		message += " Not ticked: " + strings.Join(missing, ", ") + "."
	}
	if extra != nil {
		message += " Not its manual tests: " + strings.Join(extra, ", ") + "."
	}
	return approval{}, &refused{http.StatusUnprocessableEntity, "unchecked_manual_tests", message}
}
