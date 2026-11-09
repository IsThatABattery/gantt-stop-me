import type { ReactNode } from 'react'
import { Link, NavLink } from 'react-router'
import type { AssignedGroup, AssignedTask } from '../data/api/types'
import { useMyProjects } from '../data/views/addProject'
import { useAssignedTasks } from '../data/views/assigned'
import { columnLabels, labels, runStatusLabels } from '../labels/labels'
import { runLook } from './runLook'
import { Chip, type Tone } from './Chip'
import { Icon, type IconName } from './Icon'
import { Logo } from './Logo'
import { UserMenu } from './UserMenu'

interface Item {
  label: string
  to: string
  icon: IconName
  end?: boolean
}

/** Missions listed before the rest wait behind All missions. */
const MAX_MISSIONS = 8

/** Below md the rail stacks above the page, so its long lists scroll in place instead of pushing the page down. */
const stackedCap = 'max-md:max-h-60 max-md:overflow-y-auto'

const itemBase = 'flex min-h-11 items-center rounded-lg no-underline'

/** compact: tighter, for links nested under a mission */
function RailLink({ item, className = '', compact = false }: { item: Item; className?: string; compact?: boolean }) {
  return (
    <NavLink
      to={item.to}
      end={item.end}
      className={({ isActive }) =>
        `${itemBase} ${compact ? 'gap-2 px-2' : 'gap-3 px-3'} ${className} ${
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

function Section({ title, count, children }: { title: string; count?: number; children: ReactNode }) {
  const id = `rail-${title.toLowerCase().replaceAll(' ', '-')}`
  return (
    <section aria-labelledby={id} className="mt-4 flex flex-col gap-1">
      <div className="flex items-center gap-2 px-3 pb-1">
        <h2 id={id} className="m-0 text-[11px] font-semibold tracking-[0.08em] text-text-muted uppercase">
          {title}
        </h2>
        {count !== undefined && (
          <span className="rounded-full bg-nav-active px-1.5 text-[11px] leading-[18px] font-semibold text-text-soft">{count}</span>
        )}
      </div>
      {children}
    </section>
  )
}

/** The project's pages, nested under it while it is open. */
function projectItems(projectId: string): Item[] {
  return [
    { label: labels.board, to: `/projects/${projectId}`, icon: 'board', end: true },
    { label: labels.backlog, to: `/projects/${projectId}/backlog`, icon: 'hangar' },
    { label: labels.plans, to: `/projects/${projectId}/plans`, icon: 'target' },
    { label: labels.automation, to: `/projects/${projectId}/automation`, icon: 'orbit' },
  ]
}

/** Where a task stands: a live test run first, then trouble, then a review it waits for, else its column. */
function stage(task: AssignedTask): { label: string; tone: Tone; icon?: IconName; iconClass?: string } {
  if (task.runStatus === 'running' || task.runStatus === 'queued') {
    const { tone, icon, iconClass } = runLook[task.runStatus]
    return { label: runStatusLabels[task.runStatus], tone, icon, iconClass }
  }
  if (task.needsAttention) return { label: labels.needsAttention, tone: 'critical', icon: 'alert' }
  if (task.column.key === 'user-check') return { label: labels.awaitingReview, tone: 'astronaut', icon: 'helmet' }
  // The plain column name: the rail is read at a glance, away from the board's themed headings.
  return { label: columnLabels[task.column.key].plain, tone: 'neutral' }
}

function AssignedList({ groups }: { groups: AssignedGroup[] }) {
  return groups.map(({ project, tasks }) => (
    <div key={project.id} className="flex flex-col gap-1.5 pb-2">
      <Link
        id={`rail-assigned-${project.id}`}
        to={`/projects/${project.id}`}
        title={`${project.owner}/${project.name}`}
        className="flex min-h-8 items-center gap-2 px-3 text-[13px] font-medium text-text-soft no-underline hover:text-text"
      >
        <Icon name="github" size={14} className="flex-none text-text-muted" />
        <span className="truncate">{project.name}</span>
      </Link>
      <ul aria-labelledby={`rail-assigned-${project.id}`} className="m-0 flex list-none flex-col gap-1.5 p-0">
        {tasks.map((t) => {
          const s = stage(t)
          return (
            <li key={t.id}>
              <NavLink
                to={`/projects/${project.id}/tasks/${t.id}`}
                aria-label={`#${t.issueNumber}: ${t.title}. ${s.label}`}
                className={({ isActive }) =>
                  `flex flex-col gap-2 rounded-control border bg-surface px-3 py-2.5 text-text no-underline hover:border-ion hover:text-text ${
                    isActive ? 'border-ion' : 'border-line'
                  }`
                }
              >
                <span className="flex flex-col gap-0.5">
                  <span className="font-mono text-[11px] text-text-muted">#{t.issueNumber}</span>
                  <span className="line-clamp-2 text-[13px] leading-[1.35]">{t.title}</span>
                </span>
                <Chip tone={s.tone} className="self-start">
                  {s.icon && <Icon name={s.icon} size={14} className={s.iconClass} />}
                  {s.label}
                </Chip>
              </NavLink>
            </li>
          )
        })}
      </ul>
    </div>
  ))
}

/**
 * Left rail: the missions the user opened, most recently visited first, with the open one's pages under it, then
 * their assigned tasks by mission with each one's stage.
 */
export function NavRail({ projectId }: { projectId?: string }) {
  const { data: projects = [], isPending } = useMyProjects()
  const { data: assigned = [] } = useAssignedTasks()
  const missions = projects.filter((p, i) => i < MAX_MISSIONS || String(p.id) === projectId)
  // Just added, before the list catches up: its pages still show.
  const unlisted = projectId && !isPending && !projects.some((p) => String(p.id) === projectId)

  return (
    <nav
      aria-label="Primary"
      className="box-border flex flex-[1_1_200px] flex-col gap-1 border-r border-line-soft bg-rail px-4 py-6 md:sticky md:top-0 md:h-screen md:self-start md:overflow-y-auto"
    >
      <Link to="/" className="px-2 pb-6 no-underline">
        <Logo />
      </Link>
      <Link to="/projects/new" className={`${itemBase} mb-2 gap-3 px-3 border border-line text-text-muted-strong hover:text-text`}>
        <Icon name="plus" />
        {labels.addProject}
      </Link>
      <RailLink item={{ label: labels.allProjects, to: '/projects', icon: 'hangar', end: true }} />
      {unlisted && projectItems(projectId).map((item) => <RailLink key={item.to} item={item} />)}
      {missions.length > 0 && (
        <Section title={labels.projects}>
          <ul className={`m-0 flex list-none flex-col gap-1 p-0 ${stackedCap}`}>
            {missions.map((p) => {
              const open = String(p.id) === projectId
              return (
                <li key={p.id}>
                  <Link
                    to={`/projects/${p.id}`}
                    title={`${p.owner}/${p.name}`}
                    aria-current={open ? 'true' : undefined}
                    className={`${itemBase} gap-3 px-3 ${open ? 'font-medium text-text' : 'text-text-muted-strong'} hover:text-text`}
                  >
                    <Icon name="github" className={`flex-none ${open ? 'text-ion' : ''}`} />
                    <span className="truncate">{p.name}</span>
                  </Link>
                  {open && (
                    <div className="ml-3 flex flex-col gap-1 border-l border-line-soft pl-1">
                      {projectItems(projectId).map((item) => (
                        <RailLink key={item.to} item={item} compact />
                      ))}
                    </div>
                  )}
                </li>
              )
            })}
          </ul>
        </Section>
      )}
      {assigned.length > 0 && (
        <Section title={labels.assignedToYou} count={assigned.reduce((n, g) => n + g.tasks.length, 0)}>
          <div className={`flex flex-col gap-1 ${stackedCap}`}>
            <AssignedList groups={assigned} />
          </div>
        </Section>
      )}
      <div className="mt-auto flex items-center gap-2 pt-4">
        <RailLink item={{ label: labels.settings, to: '/settings', icon: 'settings' }} className="flex-1" />
        <UserMenu placement="above" />
      </div>
    </nav>
  )
}
