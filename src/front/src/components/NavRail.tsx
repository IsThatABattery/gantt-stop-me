import { Link, NavLink } from 'react-router'
import { labels } from '../labels/labels'
import { Icon, type IconName } from './Icon'
import { Logo } from './Logo'
import { UserMenu } from './UserMenu'

interface Item {
  label: string
  to: string
  icon: IconName
  end?: boolean
}

const itemBase = 'flex min-h-11 items-center gap-3 rounded-lg px-3 no-underline'

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

/**
 * Left rail. Without a project it shows only the way back and Settings.
 * In a project it adds the board, planning and automation.
 * Backlog, sprints, agents, activity and retro come back once the API has
 * them (docs/implementation/api-gaps.md).
 */
export function NavRail({ projectId }: { projectId?: string }) {
  const items: Item[] = projectId
    ? [
        { label: labels.board, to: `/projects/${projectId}`, icon: 'board', end: true },
        { label: labels.plans, to: `/projects/${projectId}/plans`, icon: 'target' },
        { label: labels.automation, to: `/projects/${projectId}/automation`, icon: 'orbit' },
      ]
    : []

  return (
    <nav
      aria-label="Primary"
      className="box-border flex flex-[1_1_200px] flex-col gap-1 border-r border-line-soft bg-rail px-4 py-6 md:sticky md:top-0 md:h-screen md:self-start md:overflow-y-auto"
    >
      <Link to="/" className="px-2 pb-6 no-underline">
        <Logo />
      </Link>
      <Link to="/" className={`${itemBase} mb-2 border border-line text-text-muted-strong hover:text-text`}>
        <Icon name="plus" />
        {labels.addProject}
      </Link>
      {items.map((item) => (
        <RailLink key={item.to} item={item} />
      ))}
      <div className="mt-auto flex items-center gap-2">
        <RailLink item={{ label: labels.settings, to: '/settings', icon: 'settings' }} className="flex-1" />
        <UserMenu placement="above" />
      </div>
    </nav>
  )
}
