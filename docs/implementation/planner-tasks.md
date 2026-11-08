# Planner · Payload review and done

Steps 3 and 4 of planning an objective. A planning astronaut proposes payloads, the operator reviews each one, and approved payloads go to the Hangar.

- **Branch:** `feature/frontend-planner-tasks`
- **Lane:** B (planning and creating work)
- **Phase:** 4

## Design source

- `docs/ganttstopme-plan/design/mockups/PlanPayloads.dc.html` (step 3)
- `docs/ganttstopme-plan/design/mockups/PlanPayloadsDeny.dc.html` (step 3b)
- `docs/ganttstopme-plan/design/mockups/PlanDone.dc.html` (step 4)
- AGENT_BRIEF §8, steps 3–4 and "Denying with feedback"

## Routes

- `/projects/:projectId/plan/:draftId/tasks`: `TaskReviewPage` in `src/pages/planner/tasks/index.tsx`.
- `/projects/:projectId/plan/:draftId/done`: `PlanDonePage`, same file.
- Both render inside the shared `PlannerLayout`. Build only the page body.

## Files you own

- `src/pages/planner/tasks/`
- `src/data/views/plannerTasks.ts`
- `src/data/mock/plannerTasks.ts` if needed
- Tests next to these files

## Data available

- `listProposals` / `useProposals` return the draft's `TaskProposal`s in order.
- Sample draft `realtime` (approved doc, feature `f-realtime`) has 8 proposals:
  - 3 approved: PAY-60, PAY-61 and PAY-62, already in the Hangar.
  - 1 sent back.
  - 4 needing review.
  - Proposal 4 has every field filled.
- `useDraft` from `plannerDoc.ts` gives the doc, for the source-section text. Read it only; it belongs to planner-doc.
- `FeedbackForm` is the deny form. Pass `plannerLabels.taskOutcomes` and `plannerLabels.taskReasons`, with no section picker.
- `proposalStatusLabels` is in `labels.ts`.

You don't need planner-doc to be merged. Start from the `realtime` draft.

## Scope

- [ ] **Step 3 · Review payloads:**
  - **Queue column:** every proposal with status chip, estimate and title. Selecting one shows it on the right. Keyboard-reachable.
  - **Selected proposal:**
    - Status chip, "Proposal n of N · proposed by the planning astronaut", title and description.
    - Acceptance criteria, objective, mission phase, estimate, priority, suggested assignee with autonomy, depends-on.
    - The astronaut's note with its source section.
  - **Approve and add to Hangar:**
    - Creates a task in `db.tasks` with the next `PAY-n` key, in Backlog, tagged with the objective, and adds a `created` event.
    - Sets the proposal to `approved` and stores its `taskId`.
    - Refreshes `keys.proposals` and `keys.board`.
  - **Edit before approving:** the fields become editable in place, and approving afterwards sets `approved-edited`.
  - **Deny with feedback** (step 3b): opens `FeedbackForm`. Text is required. Revise sets `sent-back`, then `revised` (simulate the astronaut's return with a note on what changed). Drop sets `dropped`. A `Feedback` is recorded.
  - **Sticky progress bar:** "4 of 8 reviewed · 3 approved · 1 sent back" and "Finish review (n left)", which goes to `/done`.
- [ ] **Step 4 · Done:**
  - Summary counts: approved and now in the Hangar, sent back, still waiting.
  - "Added to the board" list with payload keys and titles.
  - Open the board / Keep reviewing payloads / View the objective (links to `/projects/:projectId/features/f-realtime/edit`).

## Out of scope

- Steps 1–2 (`feature/frontend-planner-doc`).
- A real planning agent. Revisions are simulated.
- Reordering or splitting proposals.

## Walkthrough

1. Open `/projects/orbital-nav/plan/realtime/tasks`. Proposal 4 is selected, showing all its fields.
2. Approve it. PAY-63 appears in the Hangar on the board, and the bar reads 5 of 8 reviewed.
3. Select proposal 5 and deny it with Drop. Empty text is blocked; with text it shows Dropped.
4. Edit proposal 6's estimate, then approve it. It shows "Approved · edited".
5. Finish review. The Done page lists the approved payloads with links back to the board.

## Open questions

- Which launch window do approved payloads go into: the objective's target window, or none?
- Should a revised proposal replace the original in the queue or appear next to it?

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
