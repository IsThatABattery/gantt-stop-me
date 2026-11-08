package automation

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// video answers an LLM check's recording of the browser as WebM, also once automation is off. It answers Range
// requests, so a player can seek in it, and reads from Postgres only the bytes it sends: a recording is up to 64 MiB,
// and any reader could otherwise make each tiny request load all of it.
func (a *Automation) video(w http.ResponseWriter, r *http.Request) {
	p, _, ok := a.Projects.Check(w, r, projects.Read)
	if !ok {
		return
	}
	v := &webm{ctx: r.Context(), db: a.DB}
	err := a.DB.QueryRow(r.Context(), `
		select x.id, octet_length(x.video) from test_results x join test_runs r on r.id = x.run_id join tasks t on t.id = r.task_id
		where x.id = $1 and x.run_id = $2 and t.project_id = $3 and x.video is not null`,
		pathID(r, "resultID"), pathID(r, "runID"), p.ID).Scan(&v.id, &v.size)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "No video.")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "video/webm")
	w.Header().Set("X-Content-Type-Options", "nosniff") // the bytes are the browser's, so never read as anything else
	http.ServeContent(w, r, "", time.Time{}, v)
}

// webm is a result's stored recording as an io.ReadSeeker: each Read is one query for that much of it, which the
// column's external storage serves without reading the rest. A Read failing cuts the body short.
type webm struct {
	ctx       context.Context
	db        *pgxpool.Pool
	id        int64 // the result's
	size, off int64
}

func (v *webm) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		offset += v.off
	case io.SeekEnd:
		offset += v.size
	}
	if offset < 0 {
		return 0, errors.New("seek before the start of the video")
	}
	v.off = offset
	return offset, nil
}

func (v *webm) Read(p []byte) (int, error) {
	if v.off >= v.size {
		return 0, io.EOF
	}
	var chunk []byte
	err := v.db.QueryRow(v.ctx, `select substring(video from $2 for $3) from test_results where id = $1`,
		v.id, v.off+1, len(p)).Scan(&chunk)
	if err == nil && len(chunk) == 0 {
		err = io.ErrUnexpectedEOF // the recording went
	}
	n := copy(p, chunk)
	v.off += int64(n)
	return n, err
}
