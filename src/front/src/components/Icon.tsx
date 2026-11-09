import type { ReactNode, SVGProps } from 'react'

// One drawn icon set on a 24-unit grid: a 2-unit stroke, square caps and mitred joins, so every mark shares the
// blocky geometry of the process-flow board. Fills are used only for small solid marks (a packet, a slider knob).

const checkPath = <path d="M5 12.5l4.5 4.5L19 7.5" />
const crossPath = <path d="M6 6l12 12M18 6L6 18" />

const backlog = (
  <>
    <path d="M3 13.5h5.5l1.5 3h4l1.5-3H21" />
    <path d="M5.5 4.5h13L21 13.5v6H3v-6z" />
  </>
)
const plan = (
  <>
    <path d="M5.5 3h9l4 4v14h-13z" />
    <path d="M14.5 3v4h4M9 12h6M9 16h4" />
  </>
)
const automation = <path d="M13.5 2.5L4.5 14h6.5l-1 7.5 9-11.5h-6.5z" />
/** A packet on the move: tests running. The caller animates the whole mark (runLook). */
const running = (
  <>
    <path d="M2.5 9.5h5M4.5 14.5h3" />
    <rect x="10" y="7" width="10" height="10" fill="currentColor" stroke="none" />
  </>
)
const queued = (
  <>
    <circle cx="12" cy="12" r="8.5" />
    <path d="M12 7.5v5l3 2" />
  </>
)
const alert = (
  <>
    <path d="M12 3.5l9.5 17h-19z" />
    <path d="M12 10v4.5M12 17v1" />
  </>
)
const person = (
  <>
    <path d="M9 3.5h6v6H9z" />
    <path d="M4 20.5c0-4 3.5-6.5 8-6.5s8 2.5 8 6.5" />
  </>
)
const agent = (
  <>
    <path d="M4.5 8h15v12h-15z" />
    <path d="M12 3.5V8M9.5 12.5v2M14.5 12.5v2M2 13v3M22 13v3" />
  </>
)

const paths = {
  back: <path d="M15 5l-7 7 7 7" />,
  chevronDown: <path d="M6 9.5l6 6 6-6" />,
  chevronRight: <path d="M9.5 6l6 6-6 6" />,
  menu: <path d="M3.5 6.5h17M3.5 12h17M3.5 17.5h17" />,
  search: (
    <>
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="M15.5 15.5L20.5 20.5" />
    </>
  ),
  /** Four blocks: every project */
  projects: <path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z" />,
  /** Stepped columns: the board */
  board: <path d="M3 4h5v16H3zM10 4h5v11h-5zM17 4h4v7h-4z" />,
  backlog,
  plan,
  automation,
  running,
  queued,
  /** Tests passed, done, chosen */
  check: checkPath,
  passed: checkPath,
  /** Tests failed: the code is wrong */
  failed: crossPath,
  /** Runner error: the run broke, not the code. A boxed mark, apart from the triangle and the cross. */
  error: (
    <>
      <path d="M4 4h16v16H4z" />
      <path d="M12 7.5v6M12 16v1" />
    </>
  ),
  /** Needs attention */
  alert,
  closed: (
    <>
      <path d="M4 4h16v16H4z" />
      <path d="M8 12.5l3 3 5-6" />
    </>
  ),
  /** No test run yet */
  idle: <path d="M5.5 5.5h13v13h-13z" />,
  cancelled: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M6 18L18 6" />
    </>
  ),
  /** A person checks: awaiting approval, User Check */
  person,
  agent,
  log: (
    <>
      <path d="M5 4h14v16H5z" />
      <path d="M8.5 8h7M8.5 12h7M8.5 16h4" />
    </>
  ),
  debrief: <path d="M4 20h16M7 16.5V10M12 16.5V5M17 16.5V12" />,
  /** Two sliders, as in the process-flow rail */
  settings: (
    <>
      <path d="M3 7h18M3 17h18" />
      <path d="M6 4.5h4.5v5H6zM13.5 14.5H18v5h-4.5z" fill="currentColor" />
    </>
  ),
  plus: <path d="M12 4.5v15M4.5 12h15" />,
  github: (
    <path d="M9 19c-4 1.5-4-2-6-2.5M15 22v-3.5c0-1 .1-1.4-.5-2 2.8-.3 5.5-1.4 5.5-6a4.6 4.6 0 0 0-1.3-3.2 4.2 4.2 0 0 0-.1-3.2s-1.1-.3-3.5 1.3a12 12 0 0 0-6.2 0C6.5 2.8 5.4 3.1 5.4 3.1a4.2 4.2 0 0 0-.1 3.2A4.6 4.6 0 0 0 4 9.5c0 4.6 2.7 5.7 5.5 6-.6.6-.6 1.2-.5 2V22" />
  ),
  lock: (
    <>
      <path d="M4.5 10.5h15v10h-15z" />
      <path d="M8 10.5V7a4 4 0 0 1 8 0v3.5" />
    </>
  ),
  chat: <path d="M4 4.5h16v11H9.5L4 20z" />,
  close: crossPath,
  pause: <path d="M8.5 5v14M15.5 5v14" />,
  edit: (
    <>
      <path d="M4 20h4.5L19.5 9 15 4.5 4 15.5z" />
      <path d="M12.5 7l4.5 4.5" />
    </>
  ),
  link: (
    <>
      <path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" />
      <path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" />
    </>
  ),
  pr: (
    <>
      <path d="M4 4h4v4H4zM4 16h4v4H4zM16 16h4v4h-4z" />
      <path d="M6 8v8M18 16V9.5a3 3 0 0 0-3-3h-4" />
      <path d="M13 4.5l-2 2 2 2" />
    </>
  ),
  signOut: (
    <>
      <path d="M10 4.5H4.5v15H10" />
      <path d="M14.5 8l4 4-4 4M18.5 12H9" />
    </>
  ),

  // Names from the space-mission vocabulary, kept so older call sites compile. Each draws its plain icon.
  /** @deprecated use backlog */
  hangar: backlog,
  /** @deprecated use plan */
  target: plan,
  /** @deprecated use automation */
  orbit: automation,
  /** @deprecated use running */
  liftoff: running,
  /** @deprecated use running */
  rocket: running,
  /** @deprecated use queued */
  launchpad: queued,
  /** @deprecated use agent */
  helmet: agent,
  /** @deprecated use person */
  dock: person,
} satisfies Record<string, ReactNode>

export type IconName = keyof typeof paths

interface IconProps extends Omit<SVGProps<SVGSVGElement>, 'name'> {
  name: IconName
  size?: number
}

export function Icon({ name, size = 18, strokeWidth = 2, ...rest }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="square"
      strokeLinejoin="miter"
      aria-hidden="true"
      focusable="false"
      {...rest}
    >
      {paths[name]}
    </svg>
  )
}
