package settings

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
)

// list returns the user's providers ordered by name, keys masked.
func (s *Settings) list(w http.ResponseWriter, r *http.Request) {
	rows, _ := s.db.Query(r.Context(), // CollectRows returns Query's error
		`select id, name, base_url, key_enc, models from providers where user_id = $1 order by name`,
		accounts.UserID(r.Context()))
	providers, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (provider, error) {
		var p provider
		var keyEnc []byte
		if err := row.Scan(&p.ID, &p.Name, &p.BaseURL, &keyEnc, &p.Models); err != nil {
			return p, err
		}
		key, err := s.secrets.Open(keyEnc)
		p.KeyLast4 = last4(string(key))
		return p, err
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string][]provider{"providers": providers})
}
