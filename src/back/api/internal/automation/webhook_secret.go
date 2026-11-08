package automation

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// webhookSecret replaces the secret GitHub signs the project's webhook deliveries with, and answers it this once.
func (a *Automation) webhookSecret(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Write)
	if !ok {
		return
	}
	b := make([]byte, 32)
	rand.Read(b)
	// GitHub keys its HMAC with the secret as typed in, so the receiver keys with these hex characters, not b.
	key := hex.EncodeToString(b)
	tag, err := a.DB.Exec(r.Context(), `update project_automation set webhook_secret_enc = $2 where project_id = $1`,
		p.ID, a.Secrets.Seal([]byte(key)))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusConflict, "automation_off", "Turn automation on first.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"secret": key, "payloadUrl": a.payloadURL(p.ID)})
}
