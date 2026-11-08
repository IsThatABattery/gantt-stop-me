# Mission select, finished

Finish the mission select page from Phase 1: working filters, sort, the Needs you link, and a New mission form.

- **Branch:** `feature/frontend-projects`
- **Lane:** B (planning and creating work)
- **Phase:** 5

## Design source

- `docs/ganttstopme-plan/design/mockups/Home.dc.html`
- SCREENS.md ("Mission select")

## Routes

- `/`, rendered by `src/pages/projects/ProjectSelectPage.tsx`.

## Files you own

- `src/pages/projects/`
- `src/data/views/projects.ts`
- `src/data/mock/projects.ts` if needed
- Tests next to these files

## Data available

- `listProjects` / `useProjects` return `ProjectSummary`s: the project plus `stats`, `activeSprint` and `members`.
- `Project.archived` is there (all four samples are `false`; archive one in your mock data to test the filter).
- Favorites aren't in the data model yet. Store them in localStorage for now (per user, wrapped in try/catch like `session.ts`). Or, if you'd rather keep them in data, request a `favorite` field through a shared-change PR.

## Scope

- [ ] **Filters:**
  - All: not archived.
  - Mine: the signed-in user is a member.
  - Favorites.
  - Archived.
  - Toggle buttons with `aria-pressed`, and they combine with search.
- [ ] **Favorite toggle** on each mission card. It's a real button with a label, inside or beside the card link without nesting interactive elements.
- [ ] **Sort:** Recent activity (default), Name, Launch window end (missions with no active sprint last).
- [ ] **Needs you strip:** "Review now" goes to the board of the mission that most needs attention, with the most approvals plus hails plus comms down. Hide the strip when nothing needs you.
- [ ] **New mission:**
  - A dialog with the shared `Dialog`, opened by both New mission buttons.
  - Fields: name (required) and description.
  - Creates a project in `db.projects` with empty stats, the signed-in user as member, no sprint and a color, then opens its board.
  - "Import a backlog" is out of scope; leave it out of the dialog.
- [ ] **Empty states** per filter ("No favorite missions yet").

## Out of scope

- Importing a backlog.
- Mission settings or deleting missions.
- Real per-user favorites storage.

## Walkthrough

1. Open `/`. Four missions show, sorted by recent activity.
2. Click Mine. Only missions with TV as a member show: Orbital-Nav, Lunar Relay and Deep Field.
3. Star Lunar Relay, then click Favorites. Only it shows.
4. Sort by Name. Deep Field comes first.
5. Click Review now. Orbital-Nav's board opens.
6. Create "Test Mission". Its empty board opens, and it's listed on `/`.

## Open questions

- Should favorites stay in localStorage or become a field in the data model?
- Should archived missions be hidden from Mine as well as All?

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
