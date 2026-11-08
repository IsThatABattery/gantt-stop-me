# Board

Make Mission control a working board: drag cards, filter them, and drive it from the keyboard.

- **Branch:** `feature/frontend-board`
- **Lane:** A (board and astronauts)
- **Phase:** 2

## Design source

- `docs/ganttstopme-plan/design/mockups/Main.dc.html`
- AGENT_BRIEF §5 (board, card states), §9 (motion), §10 (keyboard)
- ARCHITECTURE.md "Tasks and moves": optimistic moves and the `version` conflict check

## Routes

- `/projects/:projectId`, already rendered by `src/pages/board/BoardPage.tsx`.
- Its `<Outlet />` shows the drawer (`tasks/:taskId`) and the create dialogs (`new/feature`, `new/task`). Keep it.

## Files you own

- `src/pages/board/` (BoardPage, BoardToolbar, TaskCard, TelemetryBar, plus any new files)
- `src/data/views/board.ts`
- `src/data/mock/board.ts` if you need extra sample data
- Tests next to these files

## Data available

- `getBoard` / `useBoard` already return the project, sprint, five columns of tasks ordered by `position`, epics, features, users and agents.
- `Task.version` is there for conflict checks.
- Write moves through `useMockMutation(moveTask, [keys.board(id), keys.activity(id)])`.

## Scope

- [ ] **Drag and drop** with dnd-kit (`@dnd-kit/core` and `@dnd-kit/sortable`; add them to `package.json` in this branch). Cards move between any two columns and reorder within a column.
- [ ] **`moveTask(projectId, taskId, { column, beforeTaskId, version })`** in `board.ts`:
  - Midpoint positions.
  - Throws `ConflictError` when `version` is stale.
  - Bumps `version` and adds a `moved` event to `db.activityEvents`.
  - Moving into Closed sets status `landed`.
- [ ] **Optimistic moves.** The card moves at once, and snaps back with a visible message if the mock rejects it. To make rejection testable, add a way to force a conflict in tests.
- [ ] **Objective filter** (the select) narrows the cards to one feature.
- [ ] **Counter filters.** The three telemetry counters filter the board: Awaiting approval (User Check, not landed), Comms down (blocked), Astronauts active (agent-owned, not landed). Clicking again clears the filter, and `aria-pressed` shows the state.
- [ ] **Keyboard control** (AGENT_BRIEF §10):
  - Arrow keys move focus between cards.
  - A key moves the focused card one column left or right. Pick the keys and show them in a hint.
  - Enter opens the drawer.
- [ ] **Card states** match AGENT_BRIEF §5: hover/focus lift, astronaut-working orbit pulse, hail dot and chip, docking icon, red blocked chip with reason, landed dimmed. Most are already in `TaskCard`; check and fill gaps. Recalled/failed (red outline, title only) is new.
- [ ] **Move trail.** A short trail when a card changes column, off under `prefers-reduced-motion`.
- [ ] **Burndown.** The telemetry bar's burndown sparkline comes from the tasks instead of the static path.

## Out of scope

- Swimlanes (optional in the brief; leave for later).
- The Mission log drawer on the board. That's a follow-up once `feature/frontend-activity` merges.
- Live updates from other users.
- Launch window planning.

## Walkthrough

1. Open `/projects/orbital-nav`.
2. Drag PAY-49 from Hangar to In orbit. It stays there, and the Mission log page lists the move once activity is merged.
3. Force a conflict. The card snaps back and a message says why.
4. Pick "Realtime sync" in the objective filter. Only its cards show.
5. Click "Comms down". Only PAY-43 shows.
6. Tab to a card, use the arrows and the move key, then press Enter. The drawer opens.

## Open questions

- Which keys move a card one column? Propose a pair (for example `[` and `]`) in the PR.
- Are swimlanes (by phase, objective or assignee type) wanted in v1, or later?

## Shared changes

Shared files are frozen on this branch (see `src/front/README.md`):

- `src/data/types.ts`, `keys.ts`, `core.ts`
- `src/data/mock/fixtures.ts`, `store.ts`
- `src/labels/labels.ts`
- `src/app/`, `src/components/`, `src/styles/`

If you need a new type field, label, route or shared component, open a small PR with just that change straight into `feature/frontend`. Rebase this branch once it merges, and tell the other person so they rebase too.

## Done when

- `npm run lint`, `npm run build` and `npm test` pass in `src/front`.
- Tests cover this view's main flows. Use `renderApp(path)` from `src/test/render.tsx`, as in `src/app/router.test.tsx`.
- Every item in Scope works against the mock data, and the walkthrough above runs without console errors.
- The PR into `feature/frontend` touches only the files listed under Files you own, plus any shared-change PRs that already merged.
