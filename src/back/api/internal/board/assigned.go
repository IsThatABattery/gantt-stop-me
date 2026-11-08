package board

import (
	"net/http"

	"gantt-stop-me/api/internal/accounts"
	"gantt-stop-me/api/internal/httpx"
	"gantt-stop-me/api/internal/projects"
)

// assignedTask is a task with the column it is in, so a list across projects shows its stage.
type assignedTask struct {
	task
	Column struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"column"`
}

type assignedGroup struct {
	Project projects.Project `json:"project"`
	Tasks   []assignedTask   `json:"tasks"`
}

// assigned answers GET /api/me/tasks: the open tasks assigned to the user's GitHub account, in board order, grouped
// by project in the order GET /api/projects lists them, leaving out projects with none.
//
// shortcut: the match is on the login stored when the user linked GitHub, so after a GitHub rename nothing matches
// until they link again; refresh github_links.login when GitHub reports the new one.
//
// shortcut: assignees are as each project's last sync read them, and a project syncs only while someone has its board
// open; sync here too if assignments made on GitHub must show without opening the board.
func (b *Board) assigned(w http.ResponseWriter, r *http.Request) {
	opened, ok := b.Projects.Opened(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	groups := make([]assignedGroup, len(opened))
	index := map[int64]int{}
	ids := make([]int64, len(opened))
	for i, p := range opened {
		groups[i].Project, index[p.ID], ids[i] = p, i, p.ID
	}
	rows, err := b.DB.Query(ctx, `
		select `+taskColumns+`, t.project_id, c.key, c.name from tasks t join columns c on c.id = t.column_id
		where t.project_id = any($1) and t.state = 'open'
			and t.assignees @> jsonb_build_array(jsonb_build_object('login', (select login from github_links where user_id = $2)))
		order by c.position, t.position, t.id`, ids, accounts.UserID(ctx))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t assignedTask
		var projectID int64
		if t.task, err = b.scanTask(rows, projects.Project{}, &projectID, &t.Column.Key, &t.Column.Name); err != nil {
			httpx.Internal(w, r, err)
			return
		}
		g := &groups[index[projectID]]
		t.URL = b.issueURL(g.Project, t.IssueNumber)
		g.Tasks = append(g.Tasks, t)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r, err)
		return
	}
	out := []assignedGroup{}
	for _, g := range groups {
		if len(g.Tasks) > 0 {
			out = append(out, g)
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"projects": out})
}
