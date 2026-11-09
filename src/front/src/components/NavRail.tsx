import { useId, useState, type ReactNode } from 'react'
import { Link, NavLink, useLocation } from 'react-router'
import type { AssignedGroup, AssignedTask, Project } from '../data/api/types'
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

/** Projects listed before the rest wait behind All projects. */
const MAX_PROJECTS = 8

/** A top-level rail link, 44px: a block on hover, solid ink while it is the current page. */
function RailLink({ item }: { item: Item }) {
  return (
    <NavLink
      to={item.to}
      end={item.end}
      className={({ isActive }) =>
        `flex min-h-11 items-center gap-2.5 rounded-[3px] px-2.5 font-semibold no-underline transition-colors duration-150 ${
          isActive ? 'bg-ink text-on-ink hover:bg-ink-hover hover:text-on-ink' : 'text-ink hover:bg-block hover:text-ink'
        }`
      }
    >
      <Icon name={item.icon} />
      {item.label}
    </NavLink>
  )
}

/** One of the open project's pages, in the paper panel under its block. The current one sits on a gray block. */
function PageLink({ item }: { item: Item }) {
  return (
    <NavLink
      to={item.to}
      end={item.end}
      className={({ isActive }) =>
        `flex min-h-11 items-center gap-2.5 rounded-[2px] px-2.5 text-sm no-underline transition-colors duration-150 ${
          isActive ? 'bg-block font-bold text-ink hover:text-ink' : 'font-semibold text-ink-2 hover:bg-ink/8 hover:text-ink'
        }`
      }
    >
      <Icon name={item.icon} size={16} />
      {item.label}
    </NavLink>
  )
}

/** The project's pages, nested under it while it is open. */
function projectItems(projectId: string): Item[] {
  return [
    { label: labels.board, to: `/projects/${projectId}`, icon: 'board', end: true },
    { label: labels.backlog, to: `/projects/${projectId}/backlog`, icon: 'backlog' },
    { label: labels.plans, to: `/projects/${projectId}/plans`, icon: 'plan' },
    { label: labels.automation, to: `/projects/${projectId}/automation`, icon: 'automation' },
  ]
}

function PagePanel({ projectId, attached }: { projectId: string; attached: boolean }) {
  return (
    <div className={`flex flex-col gap-0.5 bg-paper p-1 ${attached ? 'rounded-b-[3px]' : 'rounded-[3px]'}`}>
      {projectItems(projectId).map((item) => (
        <PageLink key={item.to} item={item} />
      ))}
    </div>
  )
}

function Section({ title, count, children }: { title: string; count?: number; children: ReactNode }) {
  const id = `rail-${title.toLowerCase().replaceAll(' ', '-')}`
  return (
    <section aria-labelledby={id} className="flex flex-col gap-2">
      <div className="mx-1 flex items-center gap-2">
        <h2 id={id} className="m-0 text-[13px] font-bold text-ink-2">
          {title}
        </h2>
        {count !== undefined && (
          <span className="min-w-5 rounded-[2px] border border-ink-2 px-1 text-center text-[11px] leading-4 font-bold text-ink-2">{count}</span>
        )}
      </div>
      {children}
    </section>
  )
}

/** A project as a solid block: owner small, name bold. The open one is filled ink, with its pages attached below. */
function ProjectBlock({ project, open }: { project: Project; open: boolean }) {
  return (
    <Link
      to={`/projects/${project.id}`}
      title={`${project.owner}/${project.name}`}
      aria-current={open ? 'true' : undefined}
      className={`flex min-h-[52px] flex-col justify-center px-3 py-[7px] no-underline transition-colors duration-150 ${
        open ? 'rounded-t-[3px] bg-ink text-on-ink hover:bg-ink-hover hover:text-on-ink' : 'rounded-[3px] bg-block text-ink hover:bg-block-hover hover:text-ink'
      }`}
    >
      <span className={`truncate text-xs font-medium ${open ? 'text-on-ink-2' : 'text-ink-2'}`}>
        {project.owner}
        <span className="sr-only">/</span>
      </span>
      <span className="truncate text-sm font-bold">{project.name}</span>
    </Link>
  )
}

/** Where a task stands: a live test run first, then trouble, then an approval it waits for, else its column. */
function stage(task: AssignedTask): { label: string; tone: Tone; icon?: IconName; iconClass?: string } {
  if (task.runStatus === 'running' || task.runStatus === 'queued') {
    const { tone, icon, iconClass } = runLook[task.runStatus]
    return { label: runStatusLabels[task.runStatus], tone, icon, iconClass }
  }
  if (task.needsAttention) return { label: labels.needsAttention, tone: 'signal', icon: 'alert' }
  if (task.column.key === 'user-check') return { label: labels.awaitingApproval, tone: 'approve', icon: 'person' }
  return { label: columnLabels[task.column.key].name, tone: 'neutral' }
}

function AssignedList({ groups }: { groups: AssignedGroup[] }) {
  return groups.map(({ project, tasks }) => (
    <div key={project.id} className="flex flex-col gap-1.5">
      <Link
        id={`rail-assigned-${project.id}`}
        to={`/projects/${project.id}`}
        title={`${project.owner}/${project.name}`}
        className="mx-1 flex min-h-8 items-center gap-1.5 self-start text-xs font-bold text-ink no-underline hover:text-ink hover:underline"
      >
        <Icon name="github" size={14} className="flex-none text-ink-2" />
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
                  `flex flex-col gap-1.5 rounded-[3px] bg-paper px-2.5 pb-2 text-ink no-underline transition-colors duration-150 hover:border-ink hover:text-ink ${
                    isActive ? 'border border-t-[3px] border-ink pt-1.5' : 'border border-line pt-2'
                  }`
                }
              >
                <span className="flex items-center justify-between gap-2">
                  <span className="font-mono text-xs font-semibold text-ink-2">#{t.issueNumber}</span>
                  <Chip tone={s.tone} icon={s.icon} iconClassName={s.iconClass} className="h-[22px] px-1.5 text-[11px]">
                    {s.label}
                  </Chip>
                </span>
                <span className="line-clamp-2 text-[13px] leading-[1.3] font-semibold">{t.title}</span>
              </NavLink>
            </li>
          )
        })}
      </ul>
    </div>
  ))
}

/**
 * The blocky left rail: the product mark, the projects the user opened (most recently visited first) as blocks with
 * the open one in ink and its pages under it, their assigned tasks by project with each one's stage, then Settings
 * and the account at the foot. Below md it is a bar with a Menu button that opens the rest above the page.
 */
export function NavRail({ projectId }: { projectId?: string }) {
  const { data: projects = [], isPending } = useMyProjects()
  const { data: assigned = [] } = useAssignedTasks()
  const { pathname } = useLocation()
  // Open on one page only: following a link closes the menu.
  const [openOn, setOpenOn] = useState<string | null>(null)
  const menuOpen = openOn === pathname
  const bodyId = useId()
  const listed = projects.filter((p, i) => i < MAX_PROJECTS || String(p.id) === projectId)
  // Just added, before the list catches up: its pages still show.
  const unlisted = projectId && !isPending && !projects.some((p) => String(p.id) === projectId)

  return (
    <nav
      aria-label="Primary"
      className="box-border flex flex-col border-rule bg-rail max-md:border-b md:sticky md:top-0 md:h-screen md:overflow-y-auto md:border-r"
    >
      <div className="flex items-center justify-between gap-3 px-3.5 py-2 md:pt-[18px] md:pb-0">
        <Link to="/" className="flex min-h-11 items-center rounded-[3px] px-1 no-underline">
          <Logo />
        </Link>
        <button
          type="button"
          aria-expanded={menuOpen}
          aria-controls={bodyId}
          onClick={() => setOpenOn(menuOpen ? null : pathname)}
          className="flex min-h-11 cursor-pointer items-center gap-2 rounded-control border-[1.5px] border-ink bg-paper px-3 font-[inherit] text-sm font-semibold text-ink hover:bg-ink/8 md:hidden"
        >
          <Icon name={menuOpen ? 'close' : 'menu'} size={16} />
          Menu
        </button>
      </div>

      <div id={bodyId} className={`flex-1 flex-col gap-6 px-3.5 pt-4 pb-3.5 md:flex md:pt-6 ${menuOpen ? 'flex' : 'hidden'}`}>
        <div className="flex flex-col gap-2">
          {listed.length > 0 && (
            <Section title={labels.projects}>
              <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
                {listed.map((p) => {
                  const open = String(p.id) === projectId
                  return (
                    <li key={p.id} className="flex flex-col">
                      <ProjectBlock project={p} open={open} />
                      {open && <PagePanel projectId={projectId} attached />}
                    </li>
                  )
                })}
              </ul>
            </Section>
          )}
          {unlisted && <PagePanel projectId={projectId} attached={false} />}
          <div className="flex flex-col">
            <RailLink item={{ label: labels.allProjects, to: '/projects', icon: 'projects', end: true }} />
            <RailLink item={{ label: labels.addProject, to: '/projects/new', icon: 'plus' }} />
          </div>
        </div>

        {assigned.length > 0 && (
          <Section title={labels.assignedToYou} count={assigned.reduce((n, g) => n + g.tasks.length, 0)}>
            <div className="flex flex-col gap-3">
              <AssignedList groups={assigned} />
            </div>
          </Section>
        )}

        <div className="mt-auto flex flex-col gap-1 border-t border-rule pt-3">
          <RailLink item={{ label: labels.settings, to: '/settings', icon: 'settings' }} />
          <UserMenu placement="above" showEmail />
        </div>
      </div>
    </nav>
  )
}
