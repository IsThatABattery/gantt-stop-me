# Create and edit forms

One shared form for objectives and payloads, used as a short create dialog and as a full edit page.

- **Branch:** `feature/frontend-forms`
- **Lane:** B (planning and creating work)
- **Phase:** 2

This branch covers four mockups, an exception to "one view per branch". All four use one form, and splitting it would make two branches edit the same files.

## Design source

- `docs/ganttstopme-plan/design/mockups/NewObjective.dc.html` and `NewPayload.dc.html` (the create dialog, `kind` = feature or task)
- `docs/ganttstopme-plan/design/mockups/ObjectiveEditor.dc.html` and `PayloadEditor.dc.html` (the edit page)
- AGENT_BRIEF §7 (the field table for objective vs payload)

## Routes

All are already registered, with stubs in `src/pages/forms/index.tsx`:
- `/projects/:projectId/new/feature` and `/new/task`: dialogs over the board, using the shared `Dialog`.
- `/projects/:projectId/features/:featureId/edit` and `/tasks/:taskId/edit`: full pages in the app shell.

The board toolbar already links to the two dialogs, and "Manage objectives" opens the first objective's edit page. The drawer will link "Edit payload" to the task edit page.

## Files you own

- `src/pages/forms/`
- `src/data/views/forms.ts`
- `src/data/mock/forms.ts` if needed
- Tests next to these files

## Data available

- `getFormOptions` / `useFormOptions`: epics, features, project members, agents and sprints, for the selects.
- `getFeature` / `useFeature`: one feature.
- Add `getTask` here for the payload edit page.
- `formLabels` in `labels.ts` has every per-kind string (headings, intros, field labels, placeholders, criteria help).
- `priorityLabels` and `autonomyLabels` are there too.

## Scope

- [ ] **One form component**, parameterised by `kind: 'feature' | 'task'`:

  | Field | Objective | Payload |
  | --- | --- | --- |
  | Name | Name | Title |
  | First select | Mission phase (epic) | Objective (parent feature) |
  | Second select | Owner | Assignee (operator or astronaut) |
  | Extra fields | — | Estimate (points), priority |
  | Shared | Description, target launch window (or Not scheduled), tags | same |

  Tags are plain chips that you add and remove; they aren't themed.
- [ ] **Create dialog:** essentials only, as in the mockup.
  - **Create** validates (name required, estimate 0 or more), writes `db.features` or `db.tasks`, and refreshes `keys.board` and `keys.formOptions`.
    - A new task gets the next `PAY-n` key, goes in Backlog with status `queued`, and adds a `created` event.
    - A new feature gets the next `OBJ-n` key.
  - On success, go to the new item's edit page.
  - **Cancel** closes back to the board.
  - The objective dialog also has **Plan with an astronaut**, linking to `/projects/:projectId/plan/realtime-briefing/briefing` until drafts can be created.
- [ ] **Edit page:**
  - Breadcrumb, eyebrow (e.g. `OBJECTIVE · OBJ-7 · MISSION PHASE: CORE`), and Archive / Cancel / Save changes.
  - **Basics card** with the shared fields.
  - **Criteria checklist** (success criteria for an objective, acceptance for a payload): add, edit, remove, mark met, plus the "2 / 3 met" count.
  - **Astronauts card:** autonomy as `RadioCard`s, with "Inherit mission default" or "Inherit from objective" (`null`) plus the three levels.
  - **Right-hand list:**
    - For an objective: its payloads with landed progress and state chips, plus "New payload".
    - For a payload: its modules with done progress, plus "Add module".
  - **Save** writes and bumps `Task.version`. **Archive** sets an objective's status to `archived`.
- [ ] **Not found:** a missing feature or task id shows a clear message.

## Out of scope

- "Link PR" and "Add existing" (they need real GitHub data); show them disabled.
- A list page of all objectives.
- The planning flow itself (`feature/frontend-planner-doc` and `-tasks`).

## Walkthrough

1. On the board, click New payload. Fill in a title, pick Realtime sync, and assign Astronaut-1. Create opens the new payload's edit page, and the card is in Hangar on the board.
2. Click New objective, then Plan with an astronaut. It lands on planner step 1.
3. Open "Manage objectives" (Auth service). Add a success criterion, mark it met, and save. The count updates.
4. Open `/projects/orbital-nav/tasks/t-41/edit`. Three acceptance criteria (2 met) and four modules show.
5. Open `/projects/orbital-nav/features/nope/edit`. A not-found message shows.

## Open questions

- "Manage objectives" opens the first objective's edit page for now. Is a list of objectives wanted?
- Should creating a payload from the dialog stay on the board instead of opening the edit page?

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
