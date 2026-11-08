# Planner · Briefing and doc review

Steps 1 and 2 of planning an objective with an astronaut. A briefing astronaut interviews the operator, drafts an objective doc, and the operator approves or denies it.

- **Branch:** `feature/frontend-planner-doc`
- **Lane:** B (planning and creating work)
- **Phase:** 4

## Design source

- `docs/ganttstopme-plan/design/mockups/PlanBriefing.dc.html` (step 1)
- `docs/ganttstopme-plan/design/mockups/PlanDoc.dc.html` (step 2)
- `docs/ganttstopme-plan/design/mockups/PlanDocDeny.dc.html` (step 2b)
- AGENT_BRIEF §8, steps 1–2 and "Denying with feedback"

## Routes

- `/projects/:projectId/plan/:draftId/briefing`: `BriefingPage` in `src/pages/planner/doc/index.tsx`.
- `/projects/:projectId/plan/:draftId/doc`: `DocReviewPage`, same file.
- Both render inside the shared `PlannerLayout` (`src/app/`), which already draws the logo, breadcrumb, stepper and "Save draft and exit". Build only the page body.

## Files you own

- `src/pages/planner/doc/`
- `src/data/views/plannerDoc.ts`
- `src/data/mock/plannerDoc.ts` for the scripted interview
- Tests next to these files

`PlannerLayout` also calls `useDraft` from `plannerDoc.ts`. Keep `getDraft` / `useDraft` and their return shape.

## Data available

- `getDraft` / `useDraft` return a `FeatureDraft`: messages, sections with `topicState`, comments, `answered` and `expectedQuestions`.
- Sample drafts:
  - `realtime-briefing`: mid-interview, 2 of 8 answered, question 3 asking with three suggested answers.
  - `realtime-review`: full 8-section doc, needs review, one astronaut comment on Open questions.
- `FeedbackForm` (`src/components/`) is the deny form. Pass it `plannerLabels.docOutcomes`, `plannerLabels.docReasons` and the doc sections.
- `topicStateLabels` and `draftStatusLabels` are in `labels.ts`.

## Scope

- [ ] **Step 1 · Briefing:**
  - **Chat column:** astronaut and operator messages, "Question n of about N", suggested-answer chips that fill the answer box, a "Your answer" textarea, and Skip question / Draft the doc now / Send answer.
  - **Scripted mock:** sending an answer appends it, marks the current topic `covered` with a short body, and asks the next scripted question. Put the script in `mock/plannerDoc.ts`.
  - **Skip** marks the topic `not-yet` and moves on.
  - **Draft the doc now:**
    - Fills the doc from what's covered; uncovered topics become items under Open questions.
    - Sets status `needs-review` and version 1.
    - Goes to `/doc`.
  - **Draft doc column:** each topic with its Covered / Asking now / Not yet chip (word plus color) and a "2 of 7 topics" count.
- [ ] **Step 2 · Review doc:**
  - The whole doc: title, status chip, "Version n · drafted by the briefing astronaut from n answers", and each section (paragraph or list).
  - `[TARGET]` placeholders are highlighted so they're easy to spot.
  - **Decision panel:**
    - **Approve and plan payloads:** status `approved`, creates the feature in `db.features`, goes to `/tasks`.
    - **Deny with feedback:** opens the form in place (step 2b).
    - **Edit the doc myself:** sections become editable in place, with Save.
  - **Comments panel:** each astronaut comment, linked to its section.
- [ ] **Step 2b · Deny:**
  - `FeedbackForm` with a section picker. Feedback text is required.
  - Submit records a `Feedback` in `db.feedback`.
    - Send back: status `denied`, then return to Briefing with a "revising" note.
    - Discard: status `discarded`, then back to the board.

## Out of scope

- Steps 3–4 (`feature/frontend-planner-tasks`).
- A real agent runtime. The interview is scripted.
- Creating a brand-new draft from the New objective dialog. Use the sample drafts; a `createDraft` call can come later.

## Walkthrough

1. Open `/projects/orbital-nav/plan/realtime-briefing/briefing`. Question 3 shows with three answer chips.
2. Pick "Warn and let the mover choose" and send. Conflict handling turns Covered, and question 4 appears.
3. Click Draft the doc now. Step 2 shows the doc, with uncovered topics under Open questions.
4. Open `/projects/orbital-nav/plan/realtime-review/doc`. Deny with feedback, pick "Success criteria", and send with empty text: it's blocked. Add text and send. It returns to Briefing.
5. Approve the review draft. It lands on step 3.

## Open questions

- How does a brand-new draft start from the New objective dialog? A `createDraft` call needs agreeing with feature/frontend-forms.
- What should "Save draft and exit" save mid-interview, since the mock resets on refresh?

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
