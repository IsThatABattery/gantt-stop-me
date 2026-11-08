package automation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/github"
	"gantt-stop-me/api/internal/httpx"
)

// maxDelivery is the largest webhook delivery read, far above any pull_request payload.
const maxDelivery = 5 << 20

// webhook takes a GitHub delivery for the project's repository webhook. It checks the delivery is signed with the
// project's secret before it parses anything, answers 202, and for a pull_request event applies that pull request in
// the background as a poll would. Other events, such as ping, change nothing.
func (a *Automation) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxDelivery))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "A delivery can be at most 5 MiB.")
		return
	}
	projectID, _ := strconv.ParseInt(r.PathValue("projectID"), 10, 64) // 0, which has no secret, when not a number
	ok, err := a.signed(r.Context(), projectID, body, r.Header.Get("X-Hub-Signature-256"))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "invalid_signature", "The delivery is not signed with this project's webhook secret.")
		return
	}
	if r.Header.Get("X-GitHub-Event") != "pull_request" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var in struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(body, &in); err != nil || in.Number < 1 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "A pull_request delivery needs the pull request's number.")
		return
	}
	a.deliveries.Go(func() {
		// Not the request's context: GitHub stops waiting at the 202, and the work goes on.
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := a.hint(ctx, projectID, in.Number); err != nil {
			log.Printf("webhook for project %d, pull request %d: %v", projectID, in.Number, err)
		}
	})
	w.WriteHeader(http.StatusAccepted)
}

// signed reports whether signature, an X-Hub-Signature-256 header, is "sha256=" and the hex HMAC-SHA256 of body keyed
// with the project's webhook secret. Nothing is signed for a project without a secret or with automation off.
func (a *Automation) signed(ctx context.Context, projectID int64, body []byte, signature string) (bool, error) {
	var sealed []byte
	err := a.DB.QueryRow(ctx, `select webhook_secret_enc from project_automation where project_id = $1 and webhook_secret_enc is not null`,
		projectID).Scan(&sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	key, err := a.Secrets.Open(sealed)
	if err != nil {
		return false, err
	}
	sig, prefixed := strings.CutPrefix(signature, "sha256=")
	got, err := hex.DecodeString(sig)
	if !prefixed || err != nil {
		return false, nil
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil)), nil
}

// hint applies pull request number of the project, which a delivery named, as a poll would. A delivery is only a
// hint, since deliveries can arrive late or out of order, so hint reads the pull request and its closing references
// from GitHub first. It pauses the project when GitHub refuses the owner's credential.
func (a *Automation) hint(ctx context.Context, projectID int64, number int) error {
	p, token, _, _, err := a.target(ctx, projectID)
	if errors.Is(err, ErrOff) || errors.Is(err, github.ErrNotLinked) || errors.Is(err, ErrPaused) {
		return nil
	}
	if err != nil {
		return err
	}
	before, err := records(ctx, a.DB, projectID)
	if err != nil {
		return err
	}
	pr, err := a.GitHub.PullRequest(ctx, token, p.RepoID, number)
	if err != nil {
		return a.PauseIfRejected(ctx, projectID, token, err)
	}
	v := view{pr: pr, gone: pr.State != "open"}
	if !v.gone {
		closes, err := a.GitHub.ClosingIssues(ctx, token, p.RepoID, []string{pr.NodeID})
		if err != nil {
			return a.PauseIfRejected(ctx, projectID, token, err)
		}
		v.links = closes[number]
	}
	_, err = a.apply(ctx, p, token, before, []view{v})
	return a.PauseIfRejected(ctx, projectID, token, err)
}
