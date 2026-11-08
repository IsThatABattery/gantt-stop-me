import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { Icon } from '../../components/Icon'
import { Logo } from '../../components/Logo'
import { StatusMessage } from '../../components/StatusMessage'
import { UserMenu } from '../../components/UserMenu'
import { buttonClass } from '../../components/buttonClass'
import { Select, TextInput } from '../../components/controls'
import { useMockSession, useWorkspace } from '../../data/core'
import { useFavorites, useProjects } from '../../data/views/projects'
import { labels } from '../../labels/labels'
import { FILTERS, needsYou, plural, SORTS, visibleProjects, type Filter, type Sort } from './filters'
import { NewProjectDialog } from './NewProjectDialog'
import { ProjectCard } from './ProjectCard'

const lower = (s: string) => s.toLowerCase()

/** What to say when a filter has nothing in it. */
const emptyText: Record<Filter, string> = {
  all: `No ${lower(labels.projects)} yet. Create one to get started.`,
  mine: `You are not on any ${lower(labels.projects)} yet.`,
  favorites: `No favorite ${lower(labels.projects)} yet. Star one to keep it here.`,
  archived: `No archived ${lower(labels.projects)}.`,
}

// Mission select, from Home.dc.html: search, filters, sort, favorites,
// the Needs you strip and New mission.
export function ProjectSelectPage() {
  const { data: projects, isPending, error } = useProjects()
  const { data: workspace } = useWorkspace()
  const session = useMockSession()
  const { favorites, toggle } = useFavorites(session?.userId)
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [sort, setSort] = useState<Sort>('recent')
  const [creating, setCreating] = useState(false)

  const visible = useMemo(
    () => visibleProjects(projects ?? [], { filter, query, sort, userId: session?.userId, favorites }),
    [projects, filter, query, sort, session?.userId, favorites],
  )
  const needs = useMemo(() => needsYou(projects ?? []), [projects])

  return (
    <div className="stars min-h-screen bg-background text-text">
      <header className="flex flex-wrap items-center gap-4 border-b border-line-soft bg-rail px-8 py-4">
        <Link to="/" className="mr-auto no-underline">
          <Logo />
        </Link>
        {workspace && (
          <span className="text-[13px] text-text-muted">
            {labels.workspace} · {workspace.name}
          </span>
        )}
        <Link
          to="/settings"
          aria-label={labels.settings}
          className="inline-flex size-11 items-center justify-center rounded-control border border-line text-text-muted-strong hover:text-text"
        >
          <Icon name="settings" />
        </Link>
        <UserMenu />
      </header>

      <main className="mx-auto flex max-w-[1240px] flex-col gap-7 px-8 pt-10 pb-16">
        <div className="flex flex-wrap items-end gap-4">
          <div className="flex-[1_1_360px]">
            <h1 className="m-0 font-heading text-[32px] font-semibold">{labels.selectProject}</h1>
          </div>
          <button type="button" onClick={() => setCreating(true)} className={buttonClass('primary', 'min-h-[46px] px-5')}>
            <Icon name="plus" size={16} strokeWidth={2.25} />
            {labels.newProject}
          </button>
        </div>

        {needs.top && (
          <section
            aria-label="Needs your attention"
            className="flex flex-wrap items-center gap-x-6 gap-y-3 rounded-card border border-warning bg-warning/6 px-[18px] py-3.5"
          >
            <span className="flex items-center gap-2 font-semibold text-warning">
              <Icon name="alert" size={16} />
              {labels.needsYou}
            </span>
            {needs.approval > 0 && <span>{plural(needs.approval, lower(labels.task), lower(labels.tasks))} in mission review</span>}
            {needs.hails > 0 && <span>{plural(needs.hails, lower(labels.hail), lower(labels.hails))} from an {lower(labels.agent)}</span>}
            {needs.blockedProjects > 0 && (
              <span>{plural(needs.blockedProjects, lower(labels.project), lower(labels.projects))} with {lower(labels.blocked)}</span>
            )}
            <Link
              to={`/projects/${needs.top.id}`}
              aria-label={`${labels.reviewNow}: ${needs.top.name}`}
              className="ml-auto inline-flex min-h-11 items-center px-3 font-medium no-underline"
            >
              {labels.reviewNow}
            </Link>
          </section>
        )}

        <div className="flex flex-wrap items-center gap-3">
          <div className="flex flex-[1_1_280px] flex-col">
            <label htmlFor="find" className="sr-only">
              {labels.searchProjects}
            </label>
            <TextInput
              id="find"
              type="search"
              placeholder={labels.searchProjects}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="bg-surface"
            />
          </div>
          <div role="group" aria-label="Filter" className="flex flex-wrap gap-2">
            {FILTERS.map((f) => {
              const on = filter === f.key
              return (
                <button
                  key={f.key}
                  type="button"
                  aria-pressed={on}
                  onClick={() => setFilter(f.key)}
                  className={buttonClass(on ? 'accent' : 'secondary', on ? '' : 'text-text-muted-strong')}
                >
                  {f.label}
                </button>
              )
            })}
          </div>
          <div className="flex flex-col">
            <label htmlFor="sort" className="sr-only">
              Sort by
            </label>
            <Select id="sort" className="bg-surface" value={sort} onChange={(e) => setSort(e.target.value as Sort)}>
              {SORTS.map((s) => (
                <option key={s.key} value={s.key}>
                  {s.label}
                </option>
              ))}
            </Select>
          </div>
        </div>

        {isPending ? (
          <StatusMessage title={`Loading ${lower(labels.projects)}…`} />
        ) : error ? (
          <StatusMessage title="Something went wrong" body={error.message} />
        ) : (
          <section aria-label={labels.projects} className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-5">
            {visible.map((p) => (
              <ProjectCard key={p.id} project={p} favorite={favorites.includes(p.id)} onToggleFavorite={() => toggle(p.id)} />
            ))}
            {visible.length === 0 && (
              <p role="status" className="col-span-full m-0 py-10 text-center text-text-muted-strong">
                {query.trim() ? `No ${lower(labels.projects)} match “${query.trim()}”.` : emptyText[filter]}
              </p>
            )}
            {filter !== 'archived' && (
              <button
                type="button"
                onClick={() => setCreating(true)}
                className="flex min-h-[280px] cursor-pointer flex-col items-center justify-center gap-2.5 rounded-[14px] border border-dashed border-line-strong bg-transparent p-5 font-[inherit] text-text-muted-strong hover:border-ion"
              >
                <Icon name="plus" size={28} />
                <span className="font-medium text-text">{labels.newProject}</span>
                <span className="text-[13px]">Start from scratch</span>
              </button>
            )}
          </section>
        )}
      </main>

      {creating && <NewProjectDialog onClose={() => setCreating(false)} />}
    </div>
  )
}
