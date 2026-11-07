# GanttStopMe frontend

React + TypeScript (Vite), React Router, TanStack Query and Tailwind CSS v4. The design it follows is in `ganttstopme-plan/` at the repo root (not committed).

This is Phase 1 of `ganttstopme-plan/IMPLEMENTATION_PLAN.md`: sign in, pick a mission, see the board with static sample cards. All data is mock data.

## Run

```sh
npm install
npm run dev      # hot reload at http://localhost:5173
npm run build    # typecheck and production build
npm run lint
```

In Docker, `docker compose up --build web` from the repo root serves the build at http://localhost:8080.

Sign-in accepts anything and signs you in as the sample flight director.

## Layout

| Path | What |
| --- | --- |
| `src/styles/index.css` | Tailwind theme tokens from `design/tokens.json`, starfield, reduced motion |
| `src/labels/labels.ts` | Every space display label. Code uses plain names (Project, Task, Agent…) |
| `src/data/types.ts` | Data model, plain names |
| `src/data/api.ts` | Mock data access. The one module to replace with real API calls |
| `src/data/mock/fixtures.ts` | Placeholder data from the mockups |
| `src/data/queries.ts` | TanStack Query hooks |
| `src/data/session.ts` | Mock session in localStorage |
| `src/components/` | Shared pieces: nav rail, chips, avatars, progress bar, counters, form controls |
| `src/pages/` | Login, mission select, board, placeholders |
