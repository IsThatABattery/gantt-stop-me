# Settings

Workspace settings: astronaut autonomy defaults, approval rules, the AI provider, and a simple profile.

- **Branch:** `feature/frontend-settings`
- **Lane:** A (board and astronauts)
- **Phase:** 5

## Design source

- `docs/ganttstopme-plan/design/mockups/Settings.dc.html`
- AGENT_BRIEF §6 (autonomy levels)
- Profile isn't drawn. Build it in the same section-card style.

## Routes

- `/settings`, rendered by `src/pages/settings/index.tsx` inside the app shell. The rail here has no project.
- Use in-page section navigation (e.g. `#profile`, `#autonomy`) rather than new routes, so `router.tsx` doesn't change.

## Files you own

- `src/pages/settings/`
- `src/data/views/settings.ts`
- `src/data/mock/settings.ts` if needed
- Tests next to these files

## Data available

- `getSettings` / `useSettings` return `WorkspaceSettings`:
  - `defaultAutonomy`, `freeFlightGrantedBy`, `agentTaskCap`
  - four `approvalRules`
  - `provider` (name, `keyLast4`)
- The signed-in user comes from `useSession` (`src/data/session.ts`) and `useUser` (`src/data/core.ts`).

## Scope

- [ ] **Section nav:** Profile, Astronauts and autonomy, Operators and roles, Notifications, Integrations (`settingsLabels`).
- [ ] **Profile** (not drawn; keep it simple):
  - Name and email, saved to the user in `db.users`.
  - Change password: current, new and confirm fields. New must be 8 or more characters and match confirm. Mock only: no real check, but show errors and a "Password changed" confirmation.
- [ ] **Astronauts and autonomy:**
  - Default autonomy as three `RadioCard`s, tone `astronaut`, using `autonomyDescriptions`.
  - "Who can grant Free flight" select.
  - Concurrent astronaut payloads, a number of 1 or more.
- [ ] **Approval rules:** each rule with its description and a `Toggle`.
- [ ] **Integrations:**
  - Provider radio (OpenAI only for now).
  - API key password field. After saving, show only "•••• last4", never the full key (ARCHITECTURE.md "Provider settings").
- [ ] **Save changes / Cancel:**
  - Save writes `db.settings` through `useMockMutation` and refreshes `keys.settings`.
  - Cancel resets the form.
  - Show a saved confirmation, and warn before leaving with unsaved changes if that's easy.
- [ ] **Operators and roles** and **Notifications** sections show "Not built yet".

## Out of scope

- Operators and roles, and Notifications (not designed).
- A theme switch. There's no Plain theme in v1.
- Real auth or password hashing.

## Walkthrough

1. Open `/settings`. Profile shows TV's name and email.
2. Change the password with a mismatched confirm. An error shows. Fix it and a confirmation shows.
3. Pick Free flight as the default, turn on "Require two approvals", and save. After navigating away and back, the values stuck. They reset on a page refresh, which is expected with mock data.
4. Enter an API key and save. Only its last four characters show.

## Open questions

- When real auth arrives, can people change their email here, or only their name?
- Should autonomy settings be per workspace (as drawn) or per mission?

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
