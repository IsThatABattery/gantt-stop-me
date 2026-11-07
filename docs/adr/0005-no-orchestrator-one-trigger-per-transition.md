# No orchestrator: every Task transition has exactly one Trigger

Agents must never be coordinated by another agent, and the Board must never be moved by anything but a repository event. Each row below is a GitHub Actions event, the condition that makes it apply, the job it runs, and the one Task transition it may write. `status` is written by these jobs only; humans write it by hand solely to retire a Task. This table is the explicit state machine we chose to write instead of a TLA+ model; revisit TLA+ only if racing Board commits become a real bug.

| Event | Condition | Job | Transition |
| --- | --- | --- | --- |
| push to `main`, `BOARD.yaml` changed | a Feature has no Tasks | scoping agent | Tasks added in backlog |
| push to `main`, `BOARD.yaml` changed | a Task has no `branch` | test-writing agent | branch created and recorded |
| push to `task/<id>` by a human | Task in backlog | none | backlog → in progress |
| pull request opened or updated | any branch | run `test` commands | none |
| pull request opened or updated | head is `task/<id>` | browser agent if `acceptance: browser-agent`; update the PR's sticky comment | in progress → in review |
| review approved | head is `task/<id>` | enable squash auto-merge | none until merged |
| pull request merged | head is `task/<id>` | none | in review → closed |
| review requesting changes | head is `task/<id>` | append a Rejection with the review body as reason | in review → in progress |
| pull request closed unmerged | head is `task/<id>` | none | in review → in progress |

Scanning for "Feature without Tasks" and "Task without branch" on every Board change, rather than diffing before and after, makes a failed agent run retry on the next change for free. Because agents commit with the workflow token, their own pushes fire none of these rows.
