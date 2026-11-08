import { Link, NavLink } from 'react-router'
import { labels } from '../labels/labels'
import { Icon, type IconName } from './Icon'
import { Logo } from './Logo'

interface Item {
  label: string
  to: string
  icon: IconName
  end?: boolean
}

const itemBase = 'flex min-h-11 items-center gap-3 rounded-control px-3 no-underline'

function RailLink({ item, className = '' }: { item: Item; className?: string }) {
  return (
    <NavLink
      to={item.to}
      end={item.end}
      className={({ isActive }) =>
        `${itemBase} ${className} ${
          isActive ? 'bg-nav-active font-medium text-text hover:text-text' : 'text-text-muted-strong hover:text-text'
        }`
      }
    >
      {({ isActive }) => (
        <>
          <Icon name={item.icon} className={isActive ? 'text-ion' : ''} />
          {item.label}
        </>
      )}
    </NavLink>
  )
}

/** Left rail. Without a project it shows only the way back and Settings. */
export function NavRail({ projectId }: { projectId?: string }) {
  const base = projectId ? `/projects/${projectId}` : ''
  const items: Item[] = projectId
    ? [
        { label: labels.board, to: base, icon: 'board', end: true },
        { label: labels.backlog, to: `${base}/backlog`, icon: 'hangar' },
        { label: labels.sprints, to: `${base}/sprints`, icon: 'rocket' },
        { label: labels.agents, to: `${base}/agents`, icon: 'helmet' },
        { label: labels.activity, to: `${base}/activity`, icon: 'log' },
        { label: labels.retro, to: `${base}/retro`, icon: 'debrief' },
      ]
    : []

  return (
    <nav
      aria-label="Primary"
      className="box-border flex flex-[1_1_200px] flex-col gap-1 border-r border-line-soft bg-rail px-4 py-6"
    >
      <Link to="/" className="px-2 pb-6 no-underline">
        <Logo />
      </Link>
      <Link to="/" className={`${itemBase} mb-2 border border-line text-text-muted-strong hover:text-text`}>
        <Icon name="back" />
        {labels.allProjects}
      </Link>
      {items.map((item) => (
        <RailLink key={item.to} item={item} />
      ))}
      <RailLink item={{ label: labels.settings, to: '/settings', icon: 'settings' }} className="mt-auto" />
    </nav>
  )
}
