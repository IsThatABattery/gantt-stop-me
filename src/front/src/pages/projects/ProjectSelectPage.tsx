import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { Icon } from '../../components/Icon'
import { Logo } from '../../components/Logo'
import { StatusMessage } from '../../components/StatusMessage'
import { UserMenu } from '../../components/UserMenu'
import { buttonClass } from '../../components/buttonClass'
import { ButtonMark, Select, TextInput } from '../../components/controls'
import { PageTitle } from '../../components/PageTitle'
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

// Project select, from Home.dc.html: search, filters, sort, favorites,
// the Needs you strip and New project. On mock data, and not routed until its
// API exists (docs/implementation/api-gaps.md).
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
    <div className="stars min-h-screen bg-ground text-ink">
      <header className="flex flex-wrap items-center gap-3 border-b border-rule bg-rail px-4 py-2.5 md:px-8">
        <Link to="/" className="mr-auto no-underline">
          <Logo />
        </Link>
        {workspace && (
          <span className="text-[13px] text-ink-2">
            {labels.workspace} · {workspace.name}
          </span>
        )}
        <Link
          to="/settings"
          aria-label={labels.settings}
          className="inline-flex size-11 items-center justify-center rounded-control border-[1.5px] border-ink bg-paper text-ink transition-colors duration-150 hover:bg-ink/8 hover:text-ink active:bg-ink/15"
        >
          <Icon name="settings" />
        </Link>
        <UserMenu />
      </header>

      <main className="mx-auto flex max-w-[1240px] flex-col gap-6 px-4 pt-6 pb-16 md:px-8">
        <div className="flex flex-wrap items-end justify-between gap-x-8 gap-y-4">
          <PageTitle>{labels.selectProject}</PageTitle>
          <button type="button" onClick={() => setCreating(true)} className={buttonClass('primary', 'min-h-[52px] pl-2 pr-[18px]')}>
            <ButtonMark icon="plus" />
            {labels.newProject}
          </button>
        </div>

        {needs.top && (
          <section
            aria-label="Needs your attention"
            className="flex flex-wrap items-center gap-x-6 gap-y-2 rounded-card border border-line bg-paper py-1.5 pr-1.5 pl-1.5"
          >
            <span className="flex items-center gap-2.5 font-bold">
              <span aria-hidden="true" className="grid size-10 place-items-center rounded-[2px] bg-approve text-on-approve">
                <Icon name="person" size={20} />
              </span>
              {labels.needsYou}
            </span>
            {needs.approval > 0 && <span>{plural(needs.approval, lower(labels.task), lower(labels.tasks))} {lower(labels.awaitingApproval)}</span>}
            {needs.hails > 0 && <span>{plural(needs.hails, lower(labels.hail), lower(labels.hails))} from an {lower(labels.agent)}</span>}
            {needs.blockedProjects > 0 && (
              <span>{plural(needs.blockedProjects, lower(labels.project), lower(labels.projects))} with {lower(labels.blocked)}</span>
            )}
            <Link
              to={`/projects/${needs.top.id}`}
              aria-label={`${labels.reviewNow}: ${needs.top.name}`}
              className={buttonClass('primary', 'ml-auto')}
            >
              {labels.reviewNow}
              <Icon name="chevronRight" size={16} />
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
              className="bg-paper"
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
                  className={buttonClass(on ? 'primary' : 'secondary')}
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
            <Select id="sort" className="bg-paper" value={sort} onChange={(e) => setSort(e.target.value as Sort)}>
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
          <section aria-label={labels.projects} className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
            {visible.map((p) => (
              <ProjectCard key={p.id} project={p} favorite={favorites.includes(p.id)} onToggleFavorite={() => toggle(p.id)} />
            ))}
            {visible.length === 0 && (
              <p role="status" className="col-span-full m-0 py-10 text-center text-ink-2">
                {query.trim() ? `No ${lower(labels.projects)} match “${query.trim()}”.` : emptyText[filter]}
              </p>
            )}
            {filter !== 'archived' && (
              <button
                type="button"
                onClick={() => setCreating(true)}
                className="flex min-h-[280px] cursor-pointer flex-col items-center justify-center gap-2.5 rounded-card border-[1.5px] border-dashed border-line bg-transparent p-5 font-[inherit] text-ink-2 transition-colors duration-150 hover:border-ink hover:bg-ink/8 active:bg-ink/15"
              >
                <span aria-hidden="true" className="grid size-10 place-items-center rounded-[2px] bg-ink text-on-ink">
                  <Icon name="plus" size={20} />
                </span>
                <span className="font-bold text-ink">{labels.newProject}</span>
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
