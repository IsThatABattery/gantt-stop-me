# Frontend and API: what's missing

The frontend was built on mock data for an agent-team product: agents, epics, objectives, sprints, an activity log, planning. The API is a Kanban board over GitHub issues, with test-run automation, LLM providers and planning. This page lists what each side is missing, so the work can be split up.

Branch `feature/api-wiring` connects the screens the API already supports. The rest stay on mock data and are hidden from the app until their API exists. Their files remain in `src/front/src/pages`, and their tests are skipped with a pointer here. To bring one back, see "Mock data, for the hidden screens" in [src/front/README.md](../../src/front/README.md).

Status as of 2026-11-08, with `main` at `eed7adb` (PRs #20 planning and #21 card tests merged).

## Wired on `feature/api-wiring`

| Screen | API |
| --- | --- |
| Sign in, sign out | `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/me` |
| Sign up (new page) | `POST /api/auth/signup` |
| Add a project by repository URL (new landing page) | `POST /api/projects` |
| Board, moving cards | `GET /api/projects/{id}/board`, `POST .../tasks/{taskID}/move` |
| Task drawer: title, state, body, labels, GitHub link, test status | `GET .../tasks/{taskID}` |
| New task and edit task: title and body | `POST .../tasks`, `PATCH .../tasks/{taskID}` |
| Settings: change password | `PATCH /api/me/password` |
| Settings: link GitHub with OAuth or a token, unlink (new) | `GET /api/github/link`, `POST /api/github/token`, `DELETE /api/github` |
| Settings: LLM providers list (replaces the single key field) | `/api/providers` |

## The API exists, the UI doesn't yet

These are the quickest wins: the backend is on `main`, and only frontend work is left.

### Planning (PR #20)

- **API:** `/api/me/planning` (provider and model for the planner) and `/api/projects/{id}/plans` (start, list, show, answer, draft, edit, approve, deny, retry, discard). See [planning.md](../planning.md).
- **UI today:** the planner's briefing and doc-review screens (`pages/planner/doc`) are close to this API, but run on a scripted mock (`data/mock/plannerDoc.ts`). The only way in is a hard-coded sample draft.
- **Needed:**
  - Wire `data/views/plannerDoc.ts` to `/plans`, polling while `pending` is set.
  - Map the API's statuses and topic states (`needs_review`, `not_yet`) to the UI's (`needs-review`, `not-yet`).
  - Add a "plans" list and a "start a plan" entry point.
  - Add planning settings to Settings.
- **Not covered by the API:** the second planner step, turning the doc into task proposals (`pages/planner/tasks`). The API keeps an approved plan at `pending: "generate"`, and no agent takes that job yet.

### Task tests (PR #21)

- **API:** `GET .../tasks/{taskID}` returns `tests` and `testsError`; `PUT .../tasks/{taskID}/tests` saves them. See [board.md](../board.md).
- **UI today:** nothing. The edit page's acceptance criteria are the closest fit, but they were mock-only and are hidden.
- **Needed:** a tests list in the drawer and the edit page, with add/edit/remove, and `testsError` shown when the issue's Tests section doesn't parse.

### Automation: test runs, approve and merge

- **API:** `/api/projects/{id}/automation` (on, off, webhook secret), test runs per task, run detail with results, screenshots and video, cancel, and `POST .../tasks/{taskID}/approve`. Routes exist only when the API has `RUNNER_URL` set (compose sets it). See [automation.md](../automation.md).
- **UI today:** the board card and drawer show the latest run status only. The drawer's approval bar (approve, request changes, take over) was mock-only and is hidden.
- **Needed:**
  - A project automation settings page: choose providers and models, show the webhook payload URL and secret.
  - A runs tab in the drawer with results, logs, screenshots and video.
  - An approve button wired to approve-and-merge, including the manual-check list.
  - Explain the move refusals `run_in_progress` and `not_tested`.
- **No API for:** "request changes" and "take over".

## Neither side has it

Each of these needs an API design, endpoints with tests and docs, and then UI wiring. Screens marked *hidden* exist in the frontend on mock data.

| Feature | Frontend today | What's missing in the API |
| --- | --- | --- |
| **Project list** | Hidden. The project picker (`pages/projects`) shows cards with stats, members, favorites. The landing page is now "add by URL". | `GET /api/projects` (projects the user is a member of) and `GET /api/projects/{id}`. ARCHITECTURE.md lists this as "not in this design". |
| **User profile** | Name, initials and role (flight director or operator) appear across the app; the avatar now shows email initials. | Users have only an email. Needs a name, and a role if roles stay in the design. |
| **Workspace** | A workspace name and workspace-wide settings. | No workspace concept; providers are per user. |
| **Epics (mission phases) and features (objectives)** | Hidden. Objective create/edit pages, the objective filter on the board, epic and objective on cards. | No tables or endpoints. Could map to GitHub milestones or labels. |
| **Sprints (launch windows)** | Hidden. Sprint countdown and burndown on the board header, sprint field on forms; Sprints page was a placeholder. | No tables or endpoints. |
| **Task fields** | Hidden: assignee, priority, estimate, tags, subtasks (modules), acceptance criteria, autonomy override. | Tasks store only the issue's title, body, labels and state. Assignees and labels could come from GitHub. |
| **Agents (astronauts)** | Hidden. Roster, live panel, hails (questions to a human), trace of tool calls, pause, redirect, autonomy, recall. | No agent runtime or endpoints. The runner runs tests, not agents. |
| **Activity log** | Hidden. Per-project feed of moves, comments, approvals. | No event log. ARCHITECTURE.md lists task history as out of scope. |
| **Pull requests on a task** | Hidden. The drawer's Cargo tab, "Link PR" on the edit page. | Runs carry `prNumber`; no endpoint lists a task's PRs. |
| **Workspace settings** | Hidden. Default autonomy, who can grant Free flight, concurrent agent cap, approval rules. Roles and notifications were never designed. | None. |
| **Backlog and retro pages** | Hidden. Placeholders with no design. | None. |

## Smaller gaps

- **Sign in with GitHub and SSO**, "forgot password" and "keep me signed in" are UI only. The API has email and password sign-in only; password reset is out of scope in ARCHITECTURE.md.
- **Markdown:** task bodies show as plain text. GitHub issue bodies are Markdown, so a renderer would help.
- **Live updates:** the board refreshes when it loads; the API syncs from GitHub at most every 30 seconds. There's no push channel, so other people's moves appear on the next load.
