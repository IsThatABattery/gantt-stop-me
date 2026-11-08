# Mission log

A time-ordered feed of what operators and astronauts did on a mission. It's built as a full page here, and as a component the board can mount as its right drawer later.

- **Branch:** `feature/frontend-activity`
- **Lane:** A (board and astronauts)
- **Phase:** 3

## Design source

There's no mockup. Build from AGENT_BRIEF §5, "Right drawer: live Mission log":
- Each entry: time, actor (astronaut names violet, operator names cyan), what happened, payload ID.
- Filters planned: operators only, astronauts only, needs action.

Use the existing look as the visual guide: the Main board panels, mono timestamps, and chips. Also follow AGENT_BRIEF §9 (color meaning) and §10 (state never color-only).

## Routes

- `/projects/:projectId/activity`, rendered by `src/pages/activity/index.tsx` inside the app shell.

## Files you own

- `src/pages/activity/`
- `src/data/views/activity.ts`
- `src/data/mock/activity.ts` if you want more sample events
- Tests next to these files

## Data available

- `listActivity` / `useActivity` return this project's `ActivityEvent`s, newest first.
- Each event has `actor` (user or agent), `verb`, `taskId`, `detail` and `needsAction`.
- `activityVerbLabels` in `labels.ts` gives the display verb.
- Eight sample events on Orbital-Nav; three need action.

## Scope

- [ ] **`ActivityFeed` component** (in this folder): takes events, users, agents and tasks, and renders each entry:
  - Mono time.
  - Actor: astronaut with helmet and violet name; operator with initials and cyan name.
  - Verb and detail.
  - Payload key, linking to `/projects/:projectId/tasks/:taskId`.
  - Needs-action entries are marked with amber, an icon and a word.
- [ ] **Filters:** All, Operators only, Astronauts only, Needs action, as toggle buttons with `aria-pressed`.
- [ ] **Grouping by day** (Today, Yesterday, date) once there are older events.
- [ ] **The page:** header with eyebrow and title, filters, the feed, and an empty state per filter ("No astronaut activity yet").
- [ ] Make `ActivityFeed` usable in a narrow drawer (about 360px) as well as the full page, so the board can mount it later without changes.

## Out of scope

- Mounting the feed in the board's right drawer. That's a small follow-up once this and `feature/frontend-board` have both merged.
- Live updates.
- Writing events. Other views add them when they change things.

## Walkthrough

1. Open `/projects/orbital-nav/activity`. Eight events show, newest first.
2. Click "Astronauts only". Only Astronaut-1 to Astronaut-4 entries remain.
3. Click "Needs action". Three entries show (PR #212 docking, the hail, comms down).
4. Click PAY-41 in an entry. The board opens with the drawer for PAY-41.
5. Open `/projects/deep-field/activity`. The empty state shows.

## Open questions

- Should the full page paginate or load more once there are many events?
- Should entries be grouped per payload as well as per day?

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
