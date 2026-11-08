# Screens

Files are in `mockups/`. Frame titles come from `canvas.json`. "Links to" lists the prototype navigation in the mockups.

## Main views

| File | Screen | Purpose | Links to |
| --- | --- | --- | --- |
| `Login.dc.html` | Login | Sign in with GitHub, SSO, or email and password; request access | Mission select |
| `Home.dc.html` | Mission select | Choose a mission. "Needs you" strip across missions, search, filters (All, Mine, Favorites, Archived), sort, mission cards (state, launch window, landed bar, approval / comms-down / astronaut chips, operators), New mission | Board, Settings |
| `Main.dc.html` | Board (Mission control) | Columns, telemetry bar, objective filter, Mission log drawer, left rail | Payload drawer, astronaut console, Settings, New objective, New payload, Manage objectives, Mission select |
| `Settings.dc.html` | Settings | Default autonomy, who can grant Free flight, astronaut payload cap, approval rules, theme (Space or Plain), reduced motion | Board, astronaut console |

## Supporting screens

| File | Screen | Purpose |
| --- | --- | --- |
| `PayloadDetail.dc.html` | Payload drawer | Agent trace, cargo (PR), approval bar (Approve and land, Request changes, Take over); Edit payload link |
| `ProbeConsole.dc.html` | Astronaut console | Roster, a Hail reply, trace, controls (pause, redirect, autonomy, recall) |
| `ObjectiveEditor.dc.html` | Edit objective (and edit payload via its `kind` prop) | Full edit form plus right-hand list of payloads or modules |
| `PayloadEditor.dc.html` | Edit payload | Loads `ObjectiveEditor` in payload mode |
| `NewObjective.dc.html` | New objective dialog (and new payload via `kind`) | Short create form; "Plan with an astronaut" |
| `NewPayload.dc.html` | New payload dialog | Loads `NewObjective` in payload mode |
| `StyleTile.dc.html` | Style tile | Palette, type, chips, avatars, buttons |

## Objective planner (astronaut-assisted)

| File | Step | Notes |
| --- | --- | --- |
| `PlanBriefing.dc.html` | 1 Briefing | Interview chat plus live draft doc with topic coverage |
| `PlanDoc.dc.html` | 2 Review doc | Approve, deny, or edit; astronaut comment |
| `PlanDocDeny.dc.html` | 2b Deny doc | Feedback form with section picker |
| `PlanPayloads.dc.html` | 3 Review payloads | Queue plus selected payload; approve adds to the Hangar |
| `PlanPayloadsDeny.dc.html` | 3b Deny payload | Feedback form (revise or drop) |
| `PlanDone.dc.html` | 4 Done | Summary and links to the board |

## Flow

```
Login -> Mission select -> Board
Board -> Payload drawer -> Edit payload
Board -> Astronaut console | Settings | New objective | New payload | Manage objectives
New objective -> (manual) Edit objective
New objective -> (Plan with an astronaut) Briefing -> Review doc -> Review payloads -> Done -> Board
Review doc -> Deny -> (send back) Briefing
Review payloads -> Deny -> (revise or drop) back to the queue
```

## Layout notes

- Pages are fluid, designed at about 1440 px wide, with the left rail stacking on narrow widths. Drawers and dialogs are fixed width (drawer 560, dialog 560).
- Board columns scroll horizontally.
- Reusable pieces to extract: nav rail, status chip, avatar (operator and astronaut), payload card, counter, progress bar, feedback form, edit form shell, stepper.
