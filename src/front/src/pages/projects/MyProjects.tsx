import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import type { Project } from '../../data/api/types'
import { useMyProjects } from '../../data/views/addProject'
import { labels } from '../../labels/labels'
import { AddProjectPage } from './AddProjectPage'

const RECENT = 5

function ProjectList({ projects }: { projects: Project[] }) {
  return (
    <ul className="m-0 flex w-full list-none flex-col gap-2 p-0">
      {projects.map((p) => (
        <li key={p.id}>
          <Link
            to={`/projects/${p.id}`}
            className="flex min-h-14 items-center gap-3 rounded-[14px] border border-line bg-surface px-5 text-text no-underline hover:border-ion"
          >
            <Icon name="github" className="shrink-0" />
            <span className="min-w-0 flex-1 font-mono wrap-anywhere">
              {p.owner}/{p.name}
            </span>
            <span className="text-xs text-text-muted">{p.permission}</span>
          </Link>
        </li>
      ))}
    </ul>
  )
}

/** The landing page: the projects opened most recently, centered, or adding one when there are none. */
export function FlightDeckPage() {
  const { data: projects, isPending, error } = useMyProjects()
  if (isPending) return null
  // No GitHub link, or none opened yet: adding one explains what to do.
  if (error || projects.length === 0) return <AddProjectPage />
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-6 px-8 py-10">
      <div className="flex w-full max-w-[640px] flex-col gap-4">
        <header className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="m-0 font-heading text-[26px] font-semibold">Recent {labels.projects.toLowerCase()}</h1>
          </div>
          <Link to="/projects/new" className={buttonClass('secondary')}>
            <Icon name="plus" />
            {labels.addProject}
          </Link>
        </header>
        <ProjectList projects={projects.slice(0, RECENT)} />
        {projects.length > RECENT && (
          <Link to="/projects" className="self-center text-sm">
            {labels.allProjects}
          </Link>
        )}
      </div>
    </div>
  )
}

/** Every project the user has opened and can still see. */
export function ProjectsPage() {
  const { data: projects, isPending, error } = useMyProjects()
  return (
    <div className="flex flex-col gap-6 px-8 py-10">
      <header className="flex max-w-[640px] flex-wrap items-end justify-between gap-4">
        <h1 className="m-0 font-heading text-[26px] font-semibold">{labels.allProjects}</h1>
        <Link to="/projects/new" className={buttonClass('secondary')}>
          <Icon name="plus" />
          {labels.addProject}
        </Link>
      </header>
      <div className="max-w-[640px]">
        {isPending ? (
          <StatusMessage title={`Loading ${labels.projects.toLowerCase()}…`} />
        ) : error ? (
          <StatusMessage title="Something went wrong" body={error.message} />
        ) : projects.length === 0 ? (
          <p className="m-0 text-text-muted">No {labels.projects.toLowerCase()} opened yet.</p>
        ) : (
          <ProjectList projects={projects} />
        )}
      </div>
    </div>
  )
}
