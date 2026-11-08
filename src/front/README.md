# GanttStopMe frontend

React + TypeScript (Vite), React Router, TanStack Query and Tailwind CSS v4. The design it follows is in [`docs/ganttstopme-plan/`](../../docs/ganttstopme-plan/README.md).

The app runs on the Go API in `src/back/api`. Sign-in, sign-up, adding a project by repository URL, the board, the payload drawer, creating and editing payloads, and Settings (password, GitHub link, LLM providers) all call `/api`. Screens the API can't serve yet stay in the code on mock data but aren't routed; [docs/implementation/api-gaps.md](../../docs/implementation/api-gaps.md) lists them and what each needs.

## Run

```sh
npm install
npm run dev      # hot reload at http://localhost:5173, /api proxied to localhost:8080
npm run build    # typecheck and production build
npm run lint
npm test         # Vitest + Testing Library against a fake API (npm run test:watch to keep it running)
```

`npm run dev` needs the API on `localhost:8080`: the compose stack (`docker compose up --build` from the repo root) or the dev container's **API** launch entry. The Vite proxy presents the API's own origin, so saves pass its `Origin` check.

In Docker, `docker compose up --build web` serves the production build at http://localhost:8080, with nginx proxying `/api`.

To use the board you need an account (sign up at `/signup`) and a linked GitHub account (Settings → GitHub, with OAuth or a personal access token).

## Data

| Path | What |
| --- | --- |
| `src/data/api/client.ts` | `api(method, path, body)`, the one place that calls `/api`, and `useApiMutation` |
| `src/data/api/types.ts` | What the API sends, field for field (see `docs/*.md`) |
| `src/data/errors.ts` | `ApiError`, `NotFoundError` (404), `ConflictError` (409); `errorCode(e)` reads the API's code |
| `src/data/queryClient.ts` | The query client. Any 401 clears the signed-in user, which sends them to login |
| `src/data/session.ts` | `useMe()` from `GET /api/me`, sign in, sign up, sign out |
| `src/data/views/board.ts` | Board and optimistic moves |
| `src/data/views/tasks.ts` | One task: detail, create, edit |
| `src/data/views/addProject.ts` | Add a project by repository URL |
| `src/data/views/settings.ts` | Password, GitHub link, providers |
| `src/data/views/agentSettings.ts` | Planning agents (`/api/me/planning`) and a project's automation agents |
| `src/data/keys.ts` | All TanStack Query keys |

Pages call hooks from these modules, never `api()` directly. API errors arrive as `ApiError` with the API's `code`, so a form can put each refusal on its field.

### Mock data, for the hidden screens

The agents, activity, planner, objective, and project-list screens still read the in-memory mock (`src/data/mock/`) through their own modules in `src/data/views/` (`agents.ts`, `activity.ts`, `plannerDoc.ts`, `plannerTasks.ts`, `forms.ts`, `projects.ts`, `taskDrawer.ts`). They use `useMockSession`, `useMockProject` and `useMockProjectContext`. When a screen's API lands: wire its module to `api()`, route it in `src/app/router.tsx`, add it back to the nav rail, and unskip its tests.

## Tests

Tests render the real routes against a fake API served by MSW: `src/test/fakeApi.ts` holds its data and handlers, which follow the docs' status and error codes. `renderApp(path)` signs in as `ada@example.com` with GitHub linked. Tests read `fake` to check what was saved, `fake.requests` to check what was sent, and `failNext(route, error)` to make one request fail. Any `/api` request without a handler fails the test.

Tests for the hidden screens are `describe.skip`ped with a pointer to `api-gaps.md`.

## Layout

| Path | What |
| --- | --- |
| `src/styles/index.css` | Tailwind theme tokens from `design/tokens.json`, starfield, reduced motion |
| `src/labels/labels.ts` | Every space display label. Code uses plain names (Project, Task, Agent…) |
| `src/data/` | See [Data](#data) |
| `src/components/` | Shared pieces: nav rail, user menu, chips, avatars, form controls, drawer, dialog, tabs, and more |
| `src/app/` | Router, session guard, app shell, project layout |
| `src/pages/` | One folder per view, plus login, sign-up and not-found |
| `src/test/` | Test setup, the fake API and render helpers (`renderApp`, `renderWithProviders`) |

## Routes

| Path | View |
| --- | --- |
| `/login`, `/signup` | Sign in, create an account |
| `/` | Add a project by repository URL |
| `/settings` | Settings: account, GitHub, LLM providers, planning agents |
| `/projects/:projectId` | Board |
| `/projects/:projectId/tasks/:taskId` | Payload drawer over the board |
| `/projects/:projectId/new/task` | New payload dialog over the board |
| `/projects/:projectId/tasks/:taskId/edit` | Edit payload |
| `/projects/:projectId/automation` | Automation: on or off, and its two agents |

Project and task ids are the API's numeric ids.
