import type { ReactNode, SVGProps } from 'react'

// Line icons from the mockups, one stroke weight.
const paths = {
  back: <path d="M15 5l-7 7 7 7" />,
  board: (
    <>
      <rect x="3" y="4" width="5" height="16" rx="1.5" />
      <rect x="10" y="4" width="5" height="11" rx="1.5" />
      <rect x="17" y="4" width="4" height="7" rx="1.5" />
    </>
  ),
  hangar: (
    <>
      <path d="M3 10l9-6 9 6v10H3z" />
      <path d="M9 20v-6h6v6" />
    </>
  ),
  rocket: (
    <>
      <path d="M12 3c3 2 5 6 5 10l-2 3H9l-2-3c0-4 2-8 5-10z" />
      <circle cx="12" cy="10" r="1.6" />
      <path d="M9 19l-1 2M15 19l1 2" />
    </>
  ),
  /** A rocket on its pad: a test run waiting to start */
  launchpad: (
    <>
      <path d="M12 2.5c2.6 1.8 4.2 5.2 4.2 8.6L14.6 14H9.4l-1.6-2.9c0-3.4 1.6-6.8 4.2-8.6z" />
      <circle cx="12" cy="8.6" r="1.4" />
      <path d="M9.4 14l-1 2.5M14.6 14l1 2.5M4 20.5h16M7 20.5v-2M17 20.5v-2" />
    </>
  ),
  /** A rocket with its engine lit: a test run in progress */
  liftoff: (
    <>
      <path d="M12 2.5c2.6 1.8 4.2 5.2 4.2 8.6L14.6 14H9.4l-1.6-2.9c0-3.4 1.6-6.8 4.2-8.6z" />
      <circle cx="12" cy="8.6" r="1.4" />
      <path d="M9.4 14l-1 2.5M14.6 14l1 2.5M12 16.5v5M10 17l-.8 2.6M14 17l.8 2.6" />
    </>
  ),
  helmet: (
    <>
      <circle cx="12" cy="11" r="7" />
      <rect x="7.5" y="8.5" width="9" height="5.5" rx="2.75" />
      <path d="M9 20h6" />
    </>
  ),
  log: (
    <>
      <path d="M5 4h14v16H5z" />
      <path d="M8 8h8M8 12h8M8 16h5" />
    </>
  ),
  debrief: <path d="M4 19h16M7 16V9M12 16V5M17 16v-4" />,
  settings: (
    <>
      {/* Gear: eight square teeth around a hub */}
      <path d="M10.57 4.64L10.47 2.12L13.53 2.12L13.43 4.64L16.19 5.78L17.91 3.93L20.07 6.09L18.22 7.81L19.36 10.57L21.88 10.47L21.88 13.53L19.36 13.43L18.22 16.19L20.07 17.91L17.91 20.07L16.19 18.22L13.43 19.36L13.53 21.88L10.47 21.88L10.57 19.36L7.81 18.22L6.09 20.07L3.93 17.91L5.78 16.19L4.64 13.43L2.12 13.53L2.12 10.47L4.64 10.57L5.78 7.81L3.93 6.09L6.09 3.93L7.81 5.78Z" />
      <circle cx="12" cy="12" r="3" />
    </>
  ),
  target: (
    <>
      <circle cx="12" cy="12" r="8" />
      <circle cx="12" cy="12" r="3" />
    </>
  ),
  orbit: (
    <>
      <circle cx="12" cy="12" r="3" />
      <ellipse cx="12" cy="12" rx="10" ry="4.5" transform="rotate(-25 12 12)" />
    </>
  ),
  plus: <path d="M12 5v14M5 12h14" />,
  github: (
    <path d="M9 19c-4 1.5-4-2-6-2.5M15 22v-3.5c0-1 .1-1.4-.5-2 2.8-.3 5.5-1.4 5.5-6a4.6 4.6 0 0 0-1.3-3.2 4.2 4.2 0 0 0-.1-3.2s-1.1-.3-3.5 1.3a12 12 0 0 0-6.2 0C6.5 2.8 5.4 3.1 5.4 3.1a4.2 4.2 0 0 0-.1 3.2A4.6 4.6 0 0 0 4 9.5c0 4.6 2.7 5.7 5.5 6-.6.6-.6 1.2-.5 2V22" />
  ),
  lock: (
    <>
      <rect x="4" y="10" width="16" height="11" rx="2" />
      <path d="M8 10V7a4 4 0 0 1 8 0v3" />
    </>
  ),
  check: <path d="M5 12.5l4.5 4.5L19 7.5" />,
  alert: (
    <>
      <path d="M12 4l9 16H3z" />
      <path d="M12 10v4M12 17v.5" />
    </>
  ),
  dock: (
    <>
      <path d="M4 12h6M14 12h6" />
      <path d="M10 8v8M14 8v8" />
    </>
  ),
  chat: <path d="M5 5h14v10H9l-4 4z" />,
  close: <path d="M6 6l12 12M18 6L6 18" />,
  pause: <path d="M9 5v14M15 5v14" />,
  edit: <path d="M4 20h4L19 9l-4-4L4 16z" />,
  link: (
    <>
      <path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" />
      <path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" />
    </>
  ),
  pr: (
    <>
      <circle cx="6" cy="6" r="2" />
      <circle cx="6" cy="18" r="2" />
      <circle cx="18" cy="18" r="2" />
      <path d="M6 8v8M18 16V9a3 3 0 0 0-3-3h-4" />
    </>
  ),
  signOut: (
    <>
      <path d="M10 5H5v14h5" />
      <path d="M14 8l4 4-4 4M18 12H9" />
    </>
  ),
} satisfies Record<string, ReactNode>

export type IconName = keyof typeof paths

interface IconProps extends Omit<SVGProps<SVGSVGElement>, 'name'> {
  name: IconName
  size?: number
}

export function Icon({ name, size = 18, strokeWidth = 1.75, ...rest }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...rest}
    >
      {paths[name]}
    </svg>
  )
}
