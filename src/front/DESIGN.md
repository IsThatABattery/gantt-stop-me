---
name: GanttStopMe
description: A Kanban board over a GitHub repository, drawn as a process-flow sheet in one cool neutral family.
colors:
  ground: "#f2f4f6"
  rail: "#e6e9ed"
  block: "#d5dae0"
  block-hover: "#c9cfd6"
  tile-first: "#e4e8ec"
  tile-last: "#c8d0d8"
  tile-keyline: "rgba(255, 255, 255, 0.85)"
  paper: "#ffffff"
  keyline: "#e1e5ea"
  rule: "#c9cfd6"
  line: "#646e79"
  ink: "#14171b"
  ink-hover: "#2b3138"
  ink-2: "#48515b"
  on-ink: "#ffffff"
  on-ink-2: "#b6bec8"
  accent: "#14171b"
  on-accent: "#ffffff"
  signal: "#c8341a"
  signal-ink: "#a72a13"
  on-signal: "#ffffff"
  signal-wash: "#fcebe7"
  approve: "#ffd02a"
  on-approve: "#14171b"
  approve-ink: "#6b4c00"
  good: "#175c31"
typography:
  display:
    fontFamily: "'Big Shoulders Display Variable', 'Arial Narrow', sans-serif"
    fontSize: "58px"
    fontWeight: 900
    lineHeight: 0.86
    letterSpacing: "0.005em"
  headline:
    fontFamily: "'Big Shoulders Display Variable', 'Arial Narrow', sans-serif"
    fontSize: "44px"
    fontWeight: 900
    lineHeight: 0.86
    letterSpacing: "0.005em"
  title:
    fontFamily: "'Big Shoulders Display Variable', 'Arial Narrow', sans-serif"
    fontSize: "21px"
    fontWeight: 800
    lineHeight: 1
    letterSpacing: "0.045em"
  heading:
    fontFamily: "'Public Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "22px"
    fontWeight: 700
    lineHeight: 1.25
    letterSpacing: "-0.01em"
  body:
    fontFamily: "'Public Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.4
    fontFeature: "tnum"
  body-strong:
    fontFamily: "'Public Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 600
    lineHeight: 1.3
  label:
    fontFamily: "'Public Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 600
    lineHeight: 1.3
  chip:
    fontFamily: "'Public Sans Variable', ui-sans-serif, system-ui, sans-serif"
    fontSize: "12px"
    fontWeight: 600
    lineHeight: 1
  mono:
    fontFamily: "'Red Hat Mono Variable', ui-monospace, monospace"
    fontSize: "12.5px"
    fontWeight: 600
    lineHeight: 1.4
rounded:
  mark: "2px"
  tab: "3px"
  control: "4px"
  card: "4px"
  dialog: "6px"
spacing:
  tile-pad: "6px"
  frame-pad: "8px"
  card-gap: "8px"
  column-gap: "12px"
  control-x: "16px"
  page-x-mobile: "16px"
  page-top: "20px"
  page-x: "28px"
components:
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.on-ink}"
    typography: "{typography.body-strong}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "44px"
  button-primary-hover:
    backgroundColor: "{colors.ink-hover}"
    textColor: "{colors.on-ink}"
  button-secondary:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.body-strong}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "44px"
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "44px"
  button-danger:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.on-signal}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "44px"
  button-danger-hover:
    backgroundColor: "{colors.signal-ink}"
    textColor: "{colors.on-signal}"
  counter:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "6px 14px 6px 7px"
    height: "52px"
  counter-pressed:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.on-ink}"
  ink-tab:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.on-ink}"
    typography: "{typography.title}"
    rounded: "{rounded.tab}"
    padding: "0 10px 0 12px"
    height: "40px"
  stage-tile:
    backgroundColor: "{colors.tile-first}"
    rounded: "{rounded.card}"
    padding: "{spacing.tile-pad}"
  task-card:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.body-strong}"
    rounded: "{rounded.card}"
    padding: "3px"
  chip-neutral:
    backgroundColor: "{colors.block}"
    textColor: "{colors.ink}"
    typography: "{typography.chip}"
    rounded: "{rounded.mark}"
    padding: "0 8px"
    height: "24px"
  chip-live:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.on-accent}"
    rounded: "{rounded.mark}"
    padding: "0 8px"
    height: "24px"
  chip-signal:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.on-signal}"
    rounded: "{rounded.mark}"
    padding: "0 8px"
    height: "24px"
  chip-approve:
    backgroundColor: "{colors.approve}"
    textColor: "{colors.on-approve}"
    rounded: "{rounded.mark}"
    padding: "0 8px"
    height: "24px"
  input:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: "0 14px"
    height: "46px"
  nav-item-active:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.on-ink}"
    rounded: "{rounded.tab}"
    padding: "0 10px"
    height: "44px"
  nav-project-block:
    backgroundColor: "{colors.block}"
    textColor: "{colors.ink}"
    rounded: "{rounded.tab}"
    padding: "7px 12px"
    height: "52px"
---

# Design System: GanttStopMe

## Overview

**Creative North Star: "The Process-Flow Sheet"**

The board reads like a printed process-flow infographic: a row of stepped neutral tiles, each topped by a solid ink label tab, with tasks as white keylined cards stacked inside. Everything else in the app (the rail, settings, automation, the task drawer, dialogs, sign-in) is cut from the same sheet: ink blocks for what is current or primary, gray blocks for what is not, paper for what you read and type into. It is dense, blocky and flat, built for a team that keeps it open all day.

The palette is one cool neutral family stepped in value, and colour is spent only where a person is needed: vermilion signal when something failed or needs attention, a yellow approve fill when work waits on a person's approval. Each always comes with an icon and a word. Type is three voices with fixed jobs: condensed display caps for page titles and ink tabs, a sturdy UI sans for everything people read and click, a mono for issue numbers and branches.

Four themes (Light, the default; Dark; Space light; Space dark) repaint one structure. They change values, never roles, shapes or layout. The Space themes add a cool navy cast, a live accent (teal or cyan) and a faint static starfield behind the shell. The build refuses the category default of borderless pastel columns, floating rounded cards and a rainbow of status pills.

**Key Characteristics:**
- Stepped neutral stage tiles under solid ink tabs; the step spreads across however many columns exist.
- White paper cards with a 1px line edge and an inset keyline frame.
- Small radii (2 to 6px) everywhere; controls carry 1.5px ink edges.
- Flat at rest: depth comes from value steps and edges, not shadows.
- Colour only for a person-needed state, always paired with an icon and a word.
- Three type voices with fixed jobs; display caps never set body text.

## Colors

One cool neutral family in stepped values, a solid ink for structure and action, and two person-needed colours. The frontmatter holds the Light theme (the default); the other three themes set the same roles with the values in the theme table below.

### Primary
- **Sheet Ink** (`ink`): text, stage tabs, the primary action, the active nav block, a pressed counter, the active tab and every control's 1.5px edge. Its hover step (`ink-hover`) is a shade lighter in light themes and a shade darker in dark themes.
- **On Ink** (`on-ink`, `on-ink-2`): text and secondary text set on an ink fill, such as the project owner above its name in the open project block.
- **Live Accent** (`accent`, `on-accent`): what is happening now (tests running), links, selection caret and the focus ring. In Light and Dark it equals ink; in Space light it is a deep teal and in Space dark a cyan, so "live" gains a colour only where the theme can afford one.

### Secondary
- **Signal Vermilion** (`signal`, `on-signal`): needs attention, failed runs, errors, the danger button, the signal mark on an ink tab and a signal card's edge.
- **Signal Text and Wash** (`signal-ink`, `signal-wash`): the signal as text, and the pale wash under the reason a task needs attention or an error message.

### Tertiary
- **Approve Yellow** (`approve`, `on-approve`, `approve-ink`): awaiting a person's approval. The same yellow in all four themes. It fills the approve mark, the User Check banner, the approve counter and a 2px inner frame on cards waiting for approval; `approve-ink` is the yellow as legible text on light grounds. It is also the text selection colour.
- **Good Green** (`good`): success, carried only by an icon colour on a neutral chip, never as a fill.

### Neutral
- **Ground** (`ground`): the page behind everything; also the drawer footer and the scrim (at 80%) behind dialogs and the drawer.
- **Rail** (`rail`): the nav rail's panel.
- **Block** (`block`, `block-hover`): inactive project blocks, neutral chips, the current page inside a project, disabled fields, avatar and skeleton fills.
- **Stage Tiles** (`tile-first`, `tile-last`): the board's tiles mix from first to last across the columns; `tile-keyline` is the light inset frame inside each tile.
- **Paper** (`paper`): cards, inputs, secondary buttons, dialogs, the drawer and menus.
- **Lines** (`keyline`, `rule`, `line`): lightest first. `keyline` is the inset frame inside a card, `rule` divides regions, `line` edges every card and field at 3:1 against what it sits on.

### Theme values

Light is the frontmatter. The other themes set every role:

| Role | Dark | Space light | Space dark |
|---|---|---|---|
| ground | #111316 | #eef2f8 | #070b16 |
| rail | #181b1f | #e1e8f2 | #0b1120 |
| block | #262b31 | #cfd8e5 | #18223a |
| block-hover | #30363d | #c2ccdc | #212d4a |
| tile-first | #1b1e23 | #e0e7f1 | #0f1626 |
| tile-last | #262c33 | #c3cfdf | #1a2540 |
| tile-keyline | rgba(255,255,255,0.07) | rgba(255,255,255,0.85) | rgba(230,236,247,0.07) |
| paper | #2d3239 | #ffffff | #1f2a47 |
| keyline | #3a4048 | #dce3ed | #2b3858 |
| rule | #353b43 | #c2ccdc | #24304a |
| line | #858f9b | #5a6983 | #7584a3 |
| ink | #eef1f4 | #0f1626 | #e6ecf7 |
| ink-hover | #d3d9e0 | #24304a | #c9d4e6 |
| ink-2 | #aab3be | #435069 | #a6b2c8 |
| on-ink | #14171b | #ffffff | #070b16 |
| on-ink-2 | #48515b | #b8c3d6 | #435069 |
| accent | #eef1f4 | #095a71 | #3fd5e8 |
| on-accent | #14171b | #ffffff | #062029 |
| signal | #f0603f | #c8341a | #f6674b |
| signal-ink | #ff8f74 | #9c2811 | #ff9580 |
| on-signal | #14171b | #ffffff | #070b16 |
| signal-wash | #3d2420 | #fcebe7 | #3a1d24 |
| approve | #ffd02a | #ffd02a | #ffd02a |
| on-approve | #14171b | #0f1626 | #070b16 |
| approve-ink | #ffd02a | #6b4c00 | #ffd02a |
| good | #6fd391 | #115a2d | #4ade80 |
| starfield | none | navy dots at 8–12% | ice dots at 14–20% |

In the dark themes ink inverts to near-white, so the ink tabs, primary button and active nav become light blocks with dark text: the same structure, inverted.

### Named Rules

**The Person-Needed Rule.** Colour appears only where a person is needed: signal for needs attention, failed and errors; approve for awaiting approval. Every coloured state carries an icon and a word, never colour alone.

**The One Structure Rule.** A theme is one stylesheet that sets every role variable and nothing else. A new theme repaints values; it never adds a role, a radius or a layout change, and every text pairing must pass WCAG AA on every surface it sits on.

## Typography

**Display Font:** Big Shoulders Display (with Arial Narrow)
**Body Font:** Public Sans (with system UI sans)
**Label/Mono Font:** Red Hat Mono (with system monospace)

**Character:** A tall, condensed poster face set in heavy caps does the labelling, like the stencilled headers of a process chart; a plain, sturdy public-sector sans does all the reading; a mono marks anything that is a GitHub identifier. Body numerals are tabular everywhere.

### Hierarchy
- **Display** (900, 44px on phones, 58px from 768px, line-height 0.86, uppercase): the board's page title, the project name set huge.
- **Headline** (900, 36px on phones, 44px from 768px, line-height 0.86, uppercase): every other page title (Settings, Automation, Backlog, Projects).
- **Title** (800, 21px, line-height 1, 0.045em tracking, uppercase): the stage name on an ink tab; dialog and drawer header bars use the same voice at 22px and 0.04em.
- **Heading** (Public Sans 700, 22px, line-height 1.25, −0.01em): a task's or plan's own title inside the drawer or plan view, balanced.
- **Body** (400, 14px, line-height 1.4): all running UI text. Explanatory paragraphs sit in secondary ink at 13px and cap at 60–80ch.
- **Body Strong** (600, 14px, line-height 1.3): task titles on cards, button and nav labels.
- **Label** (600, 13px): field labels in secondary ink, stage purpose lines (400), rail section headings (700).
- **Chip** (600, 12px, line-height 1): chip words; 500 for quiet tags and counts.
- **Mono** (Red Hat Mono 600, 12.5px): issue numbers (`#36`) and branches.

### Named Rules

**The Two Places Rule.** The display face sets only page titles and ink tab bars (stage tabs, dialog and drawer headers), always in caps. Anything a person reads as a sentence is Public Sans.

**The Identifier Rule.** Issue numbers and branches are mono; nothing else is.

## Layout

The app shell is a fixed 224px nav rail on the left and the main area on the right from 768px up; below that the rail collapses to a top bar with a menu button. The rail is sticky and full height, separated from the page by a `rule` line.

Pages pad 28px horizontally (16px on phones) with 20px above. The board header stacks the title, the repo line (owner/name and task count) and the actions; from 1400px the counters and actions move up beside the title. Counters are grouped and split from the actions by a vertical `rule`.

The board is a horizontal track of stage tiles, any number of them, each at least 220px wide (272px on phones) and growing to fill, 12px apart. The track bleeds to the page edge and scrolls sideways; the page itself never scrolls sideways. Each tile is a subgrid of three rows (tab, purpose line, cards) so purposes and first cards align across columns. Inside a tile: 6px around the tab and frame, 8px inside the frame, 8px between cards.

Spacing runs on a 4px base with 6px and 3px half-steps for nested frames. Content widths cap where reading matters: 560px for dialogs and the drawer, 860px for wide dialogs, about 1160px for the automation page.

**The Any-Number-of-Columns Rule.** Nothing may depend on there being five columns: tile fills come from each tile's place in the row (0 for the first, 1 for the last), widths come from the grid's minimum, and extra columns scroll.

## Elevation & Depth

The sheet is flat. Depth at rest comes from value steps (ground, rail, tile, paper) and edges (1px line, inset keylines, 1.5px ink on controls), never from shadow. Only layers that float above the page take a soft shadow: dialogs and the drawer, and dropdown menus and the model combobox. Behind a dialog or the drawer the page dims under the ground colour at 80%.

### Shadow Vocabulary
- **Overlay** (`box-shadow: 0 20px 25px -5px rgb(0 0 0 / 0.1), 0 8px 10px -6px rgb(0 0 0 / 0.1)`): dialogs and the task drawer.
- **Menu** (`box-shadow: 0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1)`): the user menu and combobox listboxes.

### Named Rules

**The Flat Sheet Rule.** Nothing on the page casts a shadow. A shadow means the layer floats above the page and will close.

## Shapes

Blocky, with corners just softened: 4px on controls and cards, 6px on dialogs, 3px on tabs, nav blocks and inner frames, 2px on the smallest marks (chips, count boxes, the paper square in the primary button, avatars, the checkbox, the toggle knob). Nested shapes step their radius down as they nest (card 4px, inner frame 2px).

Edges carry the form. Interactive controls (buttons, counters, checkboxes, toggles, menus, dialogs) take a 1.5px ink edge. Cards and fields take a 1px `line` edge. Tiles and cards hold a second, inset frame: a light `tile-keyline` inside the tile, a `keyline` inside the card, 3px in from the edge. Tab bars sit on a 2px ink base line, with the active tab a solid ink block rounded at the top.

**The Keyline Rule.** A container that holds content gets an inset keyline frame, as in the reference infographic. The frame, not a shadow or a fill change, is what turns: an ink edge on hover and focus, a signal edge on a card that needs attention, a 2px approve frame on a card awaiting approval.

## Components

### Buttons
Square-shouldered and plain-spoken: an ink block or an ink outline.
- **Shape:** 4px radius, 1.5px edge, at least 44px tall; the board header's actions are 52px.
- **Primary:** solid ink with on-ink text at weight 700, one per view (New task, Save, Approve). On the board it leads with a 36px paper square holding its icon, set inside the ink fill.
- **Secondary:** paper with an ink edge and ink text at 600, for every other action.
- **Ghost:** no fill or edge until hovered, for quiet toolbar and list actions.
- **Danger:** signal fill, for actions that throw work away.
- **Link:** an accent text link at button height, underlined 1px, 2px on hover.
- **Hover / Focus:** 150ms colour change on the ease-out-quint curve: primary steps to ink-hover; secondary and ghost take an ink tint (8%, 15% pressed). Focus is a 2px accent outline offset 2px. Disabled drops to 45% opacity; busy shows a progress cursor and a word ("Saving…").

### Chips
- **Style:** a 24px block, 2px radius, 8px side padding, a 14px icon then a word at 12px/600, filled by tone: neutral (block), live (accent), signal, approve; good is a neutral block whose icon is green.
- **State:** a soft variant for chips beside a louder one or in running text: an outline for neutral, live and good, a wash for signal and approve. "No test run" and "Closed" are quiet words with an icon and no fill.

### Counters
A count block plus its plain word, 52px tall, paper with a 1.5px ink edge. The 36px count box fills with its tone (approve, signal or live) and an ink edge; a zero count goes hollow so the filled ones stand out. As a board filter it toggles: pressed, the whole counter fills ink and shows a close mark.

### Cards / Containers
- **Corner Style:** 4px outer, 2px inner frame.
- **Background:** paper on the stage tile.
- **Shadow Strategy:** none (see Elevation).
- **Border:** 1px `line` outer edge, 3px gap, then a 1px `keyline` inner frame. Hover and focus ink the edges. Signal cards take a signal outer and inner edge and show their reason on a signal wash; approve cards take a 2px yellow inner frame. A closed card recedes through colour (paper at 55% over the tile, secondary-ink title), never opacity, so focus and text keep their contrast. While dragging, the lifted copy tilts 1.5° with an ink edge.
- **Internal Padding:** 8px inside the frame, 6px between number row, title, chip and reason.

### Inputs / Fields
- **Style:** paper, 1px `line` edge, 4px radius, 46px tall, 14px side padding. Labels sit above at 13px/600 in secondary ink, with an optional aside link on the same row.
- **Focus:** the edge and a 1px outline turn accent.
- **Error / Disabled:** aria-invalid turns the edge signal; disabled fields go to a block fill with a rule edge and secondary text.
- **Checkbox and Toggle:** an 18px square with a 1.5px ink edge that fills ink with a check; a 48×28px toggle with a 1.5px ink edge and a square 19px knob that slides and fills ink when on.

### Navigation
- **Rail:** the rail panel holds the product mark, a Projects section, global links (All projects, Add a project), "Assigned to you", and Settings and the user at the foot. Section headings are 13px/700 in secondary ink with an outlined count box.
- **Project blocks:** 52px gray blocks with the owner small above the name in bold; the open project fills ink, with its pages attached below in a paper panel. The current page in that panel sits on a gray block at 700.
- **Rail links:** 44px, 3px radius, an icon and a word; a gray block on hover, solid ink when current.
- **Tabs:** 44px tabs over a 2px ink base line; the active tab is an ink block rounded at the top, the rest are secondary ink with an ink tint on hover. Arrow keys move between them.
- **Mobile:** below 768px the rail becomes a top bar with an outlined Menu button.

### Ink Tab (signature)
The solid ink label that tops every stage tile and panel tile: 40px tall, 3px radius, the stage name in display caps on the left, an outlined count box on the right. When the stage holds work for a person it lights a 24px signal or approve square with its icon (alert or person) before the count, labelled for screen readers. Dialogs and the drawer open with the same ink bar in display caps, inset 6px from the panel edge.

### Stage Tile (signature)
A stepped neutral fill (mixed from `tile-first` to `tile-last` by the column's place), 4px radius, 6px padding, the ink tab on top, then a purpose line and the cards inside a light inset keyline frame. User Check replaces its purpose line with a yellow approve banner (person icon and a sentence). An empty tile shows a faint outlined note ("No tasks" or "No matches").

### Motion
Colour transitions run 150ms on `cubic-bezier(0.22, 1, 0.36, 1)`. The one loop is the live mark: a small icon sliding 3px each way every 1.2s while tests run. Reduced motion turns every animation and transition off.

## Do's and Don'ts

### Do:
- **Do** set every surface, line and text colour through the theme roles (`ground`, `tile-first`, `paper`, `line`, `ink`, `signal`, `approve`…) so all four themes repaint it.
- **Do** give every coloured state an icon and a word: signal with the alert icon, approve with the person icon.
- **Do** use one ink primary action per view; everything else is a secondary outline or ghost.
- **Do** top a stage or panel tile with an ink tab and hold its content in an inset keyline frame.
- **Do** step tile fills by each tile's place in the row, so the board works for any number of columns.
- **Do** keep controls at least 44px tall, with a 1.5px ink edge and a 2px accent focus outline.
- **Do** make a closed or inactive item recede through colour (paper at 55%, secondary ink), not opacity on its text.

### Don't:
- **Don't** use colour for decoration or status that needs no person: no rainbow of status fills, no tinted columns.
- **Don't** put a shadow on anything that sits on the page; shadows belong to dialogs, the drawer and menus only.
- **Don't** round past 6px: no pill chips, no circular avatars or buttons on routed screens.
- **Don't** set sentences, buttons or labels in the display face, or set display caps in anything but the display face.
- **Don't** hard-code a theme's hex in a component, or add a role a theme would have to invent a value for.
- **Don't** hard-code five columns or a fixed board width.
