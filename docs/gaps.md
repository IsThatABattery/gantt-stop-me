# Not built yet

What the app doesn't do yet, as of 2026-11-09, and what each feature would need. Remove a row when its feature ships.

## The API has it, the UI doesn't

Only frontend work is left for these.

| Feature | What's missing |
| --- | --- |
| Display name | Edit profile (`PATCH /api/me`) sets a name, and Current user returns it. Settings has no field for it, and the app shows only the email. |
| Webhook setup | Generate webhook secret (`POST /api/projects/{id}/automation/webhook`) returns a secret and the payload URL to enter on GitHub. The Automation page doesn't offer it, so automation sees pull request changes only on its 30-second poll. |
| Cancel a test run | `POST /api/projects/{id}/runs/{runID}/cancel`. The drawer has no Cancel button, so while automation is on, a task can't be moved out of Testing until its run ends. |
| Take over a task | `POST` and `DELETE /api/projects/{id}/tasks/{taskID}/takeover` take a task from automation and give it back. The drawer has neither button, and nothing marks a task that is taken over. |
| Linked pull requests | Get task lists the pull requests that close the issue. The drawer shows only the pull request number of the latest test run. |
| Tests as a list | Set tests (`PUT /api/projects/{id}/tasks/{taskID}/tests`) writes the Tests section from a list. Today tests are edited only as Markdown in the description. |
| Deny a plan doc | `POST /api/projects/{id}/plans/{planID}/deny` sends the doc back to the interviewer with feedback. The plan view can only approve it. |
| Edit a plan doc | `PATCH /api/projects/{id}/plans/{planID}` changes doc sections. The plan view shows the doc read-only. |
| Edit a proposed task | `PATCH /api/projects/{id}/plans/{planID}/proposals/{proposalID}` changes a proposal before it is approved. The plan view can only approve or reject it. |
| Discard a plan | `DELETE /api/projects/{id}/plans/{planID}`. The plan view has no way to drop a plan. |

## Neither side has it

Each needs an API design, endpoints with tests and docs, then the UI.

| Feature | What it needs |
| --- | --- |
| Agent roster and console | A view of each agent's current task and steps, with pause, resume, redirect and recall, a per-agent autonomy level, and a way to answer an agent's question. Needs an agent runtime that works tasks: today's agents only route tests, run LLM checks and plan features. |
| Activity log | A per-project feed of moves, comments and approvals, and each task's history in the drawer. Needs an event log. |
| Features and epics | Tasks grouped under features, each with an owner, success criteria, tags and a default autonomy, and a board filter by feature. Could map to GitHub milestones or labels. |
| Sprints | Sprint dates, a countdown and burndown on the board, a target sprint on each task, and a retro page per sprint, which was never designed. |
| More task fields | Estimate, priority, subtasks, acceptance criteria and per-task autonomy. A task holds only its issue's title, body, state, labels and assignees, and the tests in its body. Labels other than the column's can't be changed from the app. |
| Pull request detail | The linked pull request's changed files and GitHub checks in the drawer. |
| Custom columns | Adding, renaming or reordering a project's columns. The database keeps columns per project, but the API and UI offer only the five defaults. |
| Project favorites and archive | Starring and archiving projects, and filters for them. The project list holds the projects you added or opened, most recent first. |
| Repository picker | Choosing one of your GitHub repositories, or creating a new one, instead of pasting a repository URL. |
| Workspace | A workspace with a name and shared settings: default agent autonomy and who may raise it, a cap on concurrent agent tasks, approval rules, roles and notifications. Today providers and planning settings are per user, automation is per project, and access comes only from each member's GitHub permission. |

## Smaller gaps

- **Sign-in page:** Continue with SSO, Forgot password and Keep me signed in do nothing. The API signs in with email and password or with GitHub, has no password reset, and keeps every session for 30 days.
- **Session ended:** the dialog that asks a user to sign in again over the page takes only a password. An account with none, as one made with Continue with GitHub, has to choose Use another account and sign in from the sign-in page, which loses what was typed on the page.
- **Markdown:** task descriptions show as plain text. Issue bodies are Markdown, so they need a renderer.
- **Live updates:** the board loads when it opens and after your own changes, so other people's moves, and the moves automation makes when a run ends, show on the next load. It needs polling or a push channel. The API syncs from GitHub when a board or task loads, at most every 30 seconds.
- **Project list:** no search or sort.
- **Drawer shortcuts:** no keys for approve, request changes or take over. The first design used A, R and T.
- **Label sets:** the app shows only plain words. The optional space-mission set (Mission, Payload, Hangar) and a way to pick it aren't built, and many sentences are still written in the screens instead of the label dictionary, so a set couldn't swap them yet.
