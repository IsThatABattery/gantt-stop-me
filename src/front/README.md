# GanttStopMe frontend

React + TypeScript (Vite), React Router, TanStack Query and Tailwind CSS v4. The design it follows is in [`docs/ganttstopme-plan/`](../../docs/ganttstopme-plan/README.md).

Phase 1 is done: sign in, pick a mission, see the board with sample cards. `feature/frontend` also holds the foundation the remaining views build on. All data is mock data.

## Run

```sh
npm install
npm run dev      # hot reload at http://localhost:5173
npm run build    # typecheck and production build
npm run lint
npm test         # Vitest + Testing Library (npm run test:watch to keep it running)
```

In Docker, `docker compose up --build web` from the repo root serves the build at http://localhost:8080.

Sign-in accepts anything and signs you in as the sample flight director. The mock data lives in memory, so a refresh starts over from the sample data.

## Working on a view

The remaining views are split into nine branches, each off `feature/frontend` and merged back into it. Each branch's scope is in `docs/implementation/<view>.md`.

| Branch | Page folder | Data module |
| --- | --- | --- |
| `feature/frontend-board` | `src/pages/board/` | `src/data/views/board.ts` |
| `feature/frontend-task-drawer` | `src/pages/task-drawer/` | `src/data/views/taskDrawer.ts` |
| `feature/frontend-agents` | `src/pages/agents/` | `src/data/views/agents.ts` |
| `feature/frontend-activity` | `src/pages/activity/` | `src/data/views/activity.ts` |
| `feature/frontend-settings` | `src/pages/settings/` | `src/data/views/settings.ts` |
| `feature/frontend-forms` | `src/pages/forms/` | `src/data/views/forms.ts` |
| `feature/frontend-planner-doc` | `src/pages/planner/doc/` | `src/data/views/plannerDoc.ts` |
| `feature/frontend-planner-tasks` | `src/pages/planner/tasks/` | `src/data/views/plannerTasks.ts` |
| `feature/frontend-projects` | `src/pages/projects/` | `src/data/views/projects.ts` |

Rules that keep the branches from conflicting:

- **Own your folder.** A view branch edits only its page folder, its data module, an optional `src/data/mock/<view>.ts` for extra sample data, and tests next to its files.
- **Shared files are frozen.** These change only through a small PR straight into `feature/frontend`, and both people rebase after it lands:
  - `src/data/types.ts`, `src/data/keys.ts`, `src/data/core.ts`
  - `src/data/mock/fixtures.ts`, `src/data/mock/store.ts`
  - `src/labels/labels.ts`
  - `src/app/` (router, layouts)
  - `src/components/`
  - `src/styles/`
- **Link, don't import.** Views reach each other by URL. Every route already exists, so a view can link to another before that view is built.
- **Write through the store.** Read and write `db` from `src/data/mock/store.ts` inside your data module only. Wrap writes in `useMockMutation(fn, [keys.board(id), …])` so other views refresh.

## Layout

| Path | What |
| --- | --- |
| `src/styles/index.css` | Tailwind theme tokens from `design/tokens.json`, starfield, reduced motion |
| `src/labels/labels.ts` | Every space display label. Code uses plain names (Project, Task, Agent…) |
| `src/data/types.ts` | Data model, plain names |
| `src/data/keys.ts` | All TanStack Query keys |
| `src/data/core.ts` | Sign-in, user, workspace and project, shared by every view |
| `src/data/views/` | One data module per view. The place to swap mock calls for the real API later |
| `src/data/mock/fixtures.ts` | Sample data from the mockups |
| `src/data/mock/store.ts` | The in-memory `db`, `delay`, errors, `useMockMutation` |
| `src/data/session.ts` | Mock session in localStorage |
| `src/components/` | Shared pieces: nav rail, chips, avatars, progress bar, counters, form controls, drawer, dialog, tabs, stepper, radio card, toggle, feedback form |
| `src/app/` | Router, session guard, app shell, project layout, planner layout |
| `src/pages/` | One folder per view, plus login, placeholders and not-found |
| `src/test/` | Test setup and render helpers (`renderApp`, `renderWithProviders`) |

## Routes

| Path | View |
| --- | --- |
| `/login` | Login |
| `/` | Mission select |
| `/settings` | Settings |
| `/projects/:projectId` | Board |
| `/projects/:projectId/tasks/:taskId` | Payload drawer over the board |
| `/projects/:projectId/new/feature`, `/new/task` | Create dialogs over the board |
| `/projects/:projectId/features/:featureId/edit`, `/tasks/:taskId/edit` | Edit pages |
| `/projects/:projectId/agents` | Astronaut console |
| `/projects/:projectId/activity` | Mission log |
| `/projects/:projectId/plan/:draftId/{briefing,doc,tasks,done}` | Planner steps 1–4 |
| `/projects/:projectId/{backlog,sprints,retro}` | Not designed yet |

Sample planner drafts: `realtime-briefing` (step 1), `realtime-review` (step 2), `realtime` (steps 3–4).
