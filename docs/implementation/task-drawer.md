# Payload drawer

The drawer that opens over the board when you click a card. It shows everything about one payload and lets a human approve astronaut output.

- **Branch:** `feature/frontend-task-drawer`
- **Lane:** A (board and astronauts)
- **Phase:** 2–3

## Design source

- `docs/ganttstopme-plan/design/mockups/PayloadDetail.dc.html`
- AGENT_BRIEF §5 (drawer contents), §6 (lifecycle, Request changes, Take over), §10 (A / R / T keys)

## Routes

- `/projects/:projectId/tasks/:taskId`, a child of the board route. It's rendered by `src/pages/task-drawer/index.tsx` inside the shared `Drawer` component, which already handles Esc and the close button.

## Files you own

- `src/pages/task-drawer/`
- `src/data/views/taskDrawer.ts`
- `src/data/mock/taskDrawer.ts` if needed
- Tests next to these files

## Data available

`getTaskDetail` / `useTaskDetail` return:
- the task, feature, epic, sprint and assignee (user or agent)
- subtasks, the linked pull request, the agent trace steps, hails, and activity events for this task

Sample with everything filled in: PAY-41 (`t-41`). It has a trace, PR #212 and four modules. PAY-45 (`t-45`) has an open hail.

## Scope

- [ ] **Header:** key in mono, a status chip using the plain status word ("Docking · Awaiting review"), assignee with autonomy, phase, objective, estimate, launch window. "Edit payload" links to `/projects/:projectId/tasks/:taskId/edit`.
- [ ] **Tabs** with the shared `Tabs` component:
  - **Brief:** description and acceptance criteria with met/unmet.
  - **Modules:** subtasks with done state and a done progress bar. Toggling done writes to `db.subtasks` and updates `Task.subtasks`.
  - **Flight log:** this task's activity events and answered hails.
  - **Cargo:** the PR (number, branch, +/− lines, files, checks).
- [ ] **Astronaut trace:** the step-by-step mono trace (number, kind, time, text) for agent-owned tasks.
- [ ] **Approval bar**, pinned through the `Drawer` `footer` prop. It shows when the task is in User Check with status `awaiting-approval` or `in-review`:
  - **Approve & land:** moves to Closed, status `landed`.
  - **Request changes:** back to In Progress, status `working`, with a required note logged as an event.
  - **Take over:** assignee becomes the signed-in user, status `working`.
  - Each writes through `useMockMutation` and refreshes `keys.board` and `keys.task`.
- [ ] **Keyboard:** A / R / T trigger the three actions while the drawer is open and focus isn't in a text field. Show the hint line from the mockup.
- [ ] **Not found:** a task that doesn't exist shows a clear message in the drawer instead of crashing.

## Out of scope

- Editing fields in the drawer. That's the edit page in `feature/frontend-forms`.
- Answering hails; the reply box lives in the astronaut console (`feature/frontend-agents`). The Flight log only shows them.
- Real PR data or GitHub links beyond the mock.

## Walkthrough

1. On the board, click PAY-41. The drawer shows the header, four modules (three done), the trace, PR #212 and the approval bar.
2. Press A. The card lands in Closed and the drawer reflects it.
3. Open PAY-42 and press R. A note is required, and the card moves back to In orbit.
4. Open PAY-51. There's no approval bar, since it isn't in User Check.
5. Press Esc. The drawer closes back to the board.

## Open questions

- Should Take over also clear the astronaut's current task, or leave the astronaut assigned as a helper?
- Should Request changes always need a note? This doc assumes yes.

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
