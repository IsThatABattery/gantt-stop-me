import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Button, ButtonMark } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { PageTitle } from '../../components/PageTitle'
import type { Permission, Project } from '../../data/api/types'
import { errorCode } from '../../data/errors'
import { useMyProjects } from '../../data/views/addProject'
import { labels } from '../../labels/labels'
import { AddProjectPage } from './AddProjectPage'
import { Notice } from './Notice'
import { plural } from './filters'

const RECENT = 6

const lower = (s: string) => s.toLowerCase()

/** What the user may do in the repository, in words. */
const access: Record<Permission, { label: string; icon: 'person' | 'edit' | 'lock' }> = {
  admin: { label: 'Admin', icon: 'person' },
  write: { label: 'Can write', icon: 'edit' },
  read: { label: 'Read only', icon: 'lock' },
}

/** A project as a block, as in the rail: owner small, name bold. It fills ink under the pointer. */
function ProjectBlock({ project: p }: { project: Project }) {
  const a = access[p.permission]
  return (
    <Link
      to={`/projects/${p.id}`}
      title={`${p.owner}/${p.name}`}
      className="group flex min-h-[92px] items-center gap-3 rounded-[3px] bg-block py-3 pr-3 pl-4 text-ink no-underline transition-colors duration-150 ease-out-quint hover:bg-ink hover:text-on-ink focus-visible:bg-ink focus-visible:text-on-ink active:bg-ink-hover"
    >
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-[13px] font-medium text-ink-2 group-hover:text-on-ink-2 group-focus-visible:text-on-ink-2">
          {p.owner}
          <span className="sr-only">/</span>
        </span>
        <span className="truncate text-lg leading-snug font-bold">{p.name}</span>
        <span className="mt-2 flex items-center gap-1.5 text-xs font-semibold text-ink-2 group-hover:text-on-ink-2 group-focus-visible:text-on-ink-2">
          <Icon name={a.icon} size={14} className="flex-none" />
          <span className="sr-only">, </span>
          {a.label}
        </span>
      </span>
      <Icon name="chevronRight" className="flex-none text-ink-2 transition-transform duration-150 group-hover:translate-x-0.5 group-hover:text-on-ink group-focus-visible:text-on-ink" />
    </Link>
  )
}

function ProjectGrid({ projects, label }: { projects: Project[]; label: string }) {
  return (
    <ul aria-label={label} className="m-0 grid list-none grid-cols-[repeat(auto-fill,minmax(250px,1fr))] gap-2 p-0">
      {projects.map((p) => (
        <li key={p.id} className="flex flex-col">
          <ProjectBlock project={p} />
        </li>
      ))}
    </ul>
  )
}

/** The ink Add a project action, at the height of the board's header actions. */
function AddProjectLink() {
  return (
    <Link to="/projects/new" className={buttonClass('primary', 'min-h-[52px] pl-2 pr-[18px]')}>
      <ButtonMark icon="plus" />
      {labels.addProject}
    </Link>
  )
}

/** The page header, as on the board: the title in display caps with a line under it, the action to the right. */
function Header({ title, sub, action }: { title: string; sub?: ReactNode; action?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-x-8 gap-y-4">
      <div className="min-w-0">
        <PageTitle>{title}</PageTitle>
        {sub && <p className="mt-2.5 mb-0 text-[15px] text-ink-2">{sub}</p>}
      </div>
      {action}
    </header>
  )
}

const page = 'flex min-w-0 flex-1 flex-col gap-6 px-4 pt-5 pb-10 md:px-7'

/** Placeholder blocks while the list loads, so the page doesn't jump. */
function LoadingGrid({ label }: { label: string }) {
  return (
    <div role="status" className="grid grid-cols-[repeat(auto-fill,minmax(250px,1fr))] gap-2">
      <span className="sr-only">{label}</span>
      {[0, 1, 2].map((i) => (
        <span key={i} aria-hidden="true" className="min-h-[92px] rounded-[3px] bg-block/60" />
      ))}
    </div>
  )
}

/** The landing page: the projects opened most recently, or adding one when there are none. */
export function FlightDeckPage() {
  const { data: projects, isPending, error } = useMyProjects()
  if (isPending)
    return (
      <div className={page}>
        <Header title={`Recent ${lower(labels.projects)}`} />
        <LoadingGrid label={`Loading ${lower(labels.projects)}…`} />
      </div>
    )
  // No GitHub link, or none opened yet: adding one explains what to do.
  if (error || projects.length === 0) return <AddProjectPage />
  return (
    <div className={page}>
      <Header
        title={`Recent ${lower(labels.projects)}`}
        sub={projects.length > RECENT ? `The ${RECENT} you opened last, most recent first` : 'Most recently opened first'}
        action={<AddProjectLink />}
      />
      <ProjectGrid projects={projects.slice(0, RECENT)} label={`Recent ${lower(labels.projects)}`} />
      {projects.length > RECENT && (
        <Link to="/projects" className={buttonClass('secondary', 'self-start')}>
          {labels.allProjects}
          <span className="font-medium text-ink-2">{projects.length}</span>
          <Icon name="chevronRight" size={16} />
        </Link>
      )}
    </div>
  )
}

/** Every project the user has opened and can still see. */
export function ProjectsPage() {
  const { data: projects, isPending, error, refetch } = useMyProjects()
  return (
    <div className={page}>
      <Header
        title={labels.allProjects}
        sub={projects && projects.length > 0 ? `${plural(projects.length, lower(labels.project), lower(labels.projects))} you opened and can still see` : undefined}
        action={<AddProjectLink />}
      />
      {isPending ? (
        <LoadingGrid label={`Loading ${lower(labels.projects)}…`} />
      ) : errorCode(error) === 'github_not_linked' ? (
        <Notice
          icon="github"
          title="Link GitHub first"
          actions={
            <Link to="/settings#github" className={buttonClass('primary')}>
              <Icon name="github" />
              {labels.linkGitHub}
            </Link>
          }
        >
          {labels.projects} are GitHub repositories, so the board needs your GitHub account to list them.
        </Notice>
      ) : error ? (
        <Notice
          role="alert"
          tone="signal"
          title={`${labels.projects} didn't load`}
          actions={
            <Button variant="primary" onClick={() => refetch()}>
              Try again
            </Button>
          }
        >
          {error.message}
        </Notice>
      ) : projects.length === 0 ? (
        <Notice icon="projects" title={`No ${lower(labels.projects)} yet`}>
          Add a GitHub repository by its URL, and its open issues become tasks on a board.
        </Notice>
      ) : (
        <ProjectGrid projects={projects} label={labels.allProjects} />
      )}
    </div>
  )
}
