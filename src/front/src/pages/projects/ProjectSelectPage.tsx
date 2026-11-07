import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { UserAvatar } from '../../components/Avatar'
import { Chip } from '../../components/Chip'
import { Eyebrow } from '../../components/Eyebrow'
import { Icon } from '../../components/Icon'
import { Logo } from '../../components/Logo'
import { ProgressBar } from '../../components/ProgressBar'
import { StatusMessage } from '../../components/StatusMessage'
import { UserMenu } from '../../components/UserMenu'
import { buttonClass } from '../../components/buttonClass'
import { Select, TextInput } from '../../components/controls'
import { useWorkspace } from '../../data/core'
import { useProjects } from '../../data/views/projects'
import type { ProjectSummary } from '../../data/types'
import { labels } from '../../labels/labels'
import { countdown, shortDate, timeAgo } from '../../lib/time'

// Matches Home.dc.html. Grid and search work; the Needs you strip, filters,
// sort and New mission are shown but inert in Phase 1.
export function ProjectSelectPage() {
  const { data: projects, isPending, error } = useProjects()
  const { data: workspace } = useWorkspace()
  const [query, setQuery] = useState('')

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!projects || !q) return projects ?? []
    return projects.filter((p) => p.name.toLowerCase().includes(q) || p.description.toLowerCase().includes(q))
  }, [projects, query])

  const needs = useMemo(() => {
    const list = projects ?? []
    return {
      approval: list.reduce((n, p) => n + p.stats.awaitingApproval, 0),
      hails: list.reduce((n, p) => n + p.stats.hails, 0),
      blockedProjects: list.filter((p) => p.stats.blocked > 0).length,
    }
  }, [projects])
  const hasNeeds = needs.approval + needs.hails + needs.blockedProjects > 0

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
            <Eyebrow>
              {labels.flightDeck} · {projects?.length ?? 0} {labels.projects}
            </Eyebrow>
            <h1 className="mt-1.5 mb-0 font-heading text-[32px] font-semibold">{labels.selectProject}</h1>
          </div>
          <button type="button" className={buttonClass('primary', 'min-h-[46px] px-5')}>
            <Icon name="plus" size={16} strokeWidth={2.25} />
            {labels.newProject}
          </button>
        </div>

        {hasNeeds && (
          <section
            aria-label="Needs your attention"
            className="flex flex-wrap items-center gap-x-6 gap-y-3 rounded-card border border-warning bg-warning/6 px-[18px] py-3.5"
          >
            <span className="flex items-center gap-2 font-semibold text-warning">
              <Icon name="alert" size={16} />
              {labels.needsYou}
            </span>
            {needs.approval > 0 && <span>{plural(needs.approval, lower(labels.task), lower(labels.tasks))} docking for review</span>}
            {needs.hails > 0 && <span>{plural(needs.hails, lower(labels.hail), lower(labels.hails))} from an {lower(labels.agent)}</span>}
            {needs.blockedProjects > 0 && (
              <span>{plural(needs.blockedProjects, lower(labels.project), lower(labels.projects))} with {lower(labels.blocked)}</span>
            )}
            <a href="#" onClick={(e) => e.preventDefault()} className="ml-auto inline-flex min-h-11 items-center px-3 font-medium no-underline">
              {labels.reviewNow}
            </a>
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
            {['All', 'Mine', 'Favorites', 'Archived'].map((f, i) => (
              <button
                key={f}
                type="button"
                aria-pressed={i === 0}
                className={buttonClass(i === 0 ? 'accent' : 'secondary', i === 0 ? '' : 'text-text-muted-strong')}
              >
                {f}
              </button>
            ))}
          </div>
          <div className="flex flex-col">
            <label htmlFor="sort" className="sr-only">
              Sort by
            </label>
            <Select id="sort" className="bg-surface">
              <option>Recent activity</option>
              <option>Name</option>
              <option>Launch window end</option>
            </Select>
          </div>
        </div>

        {isPending ? (
          <StatusMessage title={`Loading ${labels.projects.toLowerCase()}…`} />
        ) : error ? (
          <StatusMessage title="Something went wrong" body={error.message} />
        ) : (
          <section aria-label={labels.projects} className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-5">
            {visible.map((p) => (
              <ProjectCard key={p.id} project={p} />
            ))}
            {visible.length === 0 && (
              <p className="col-span-full m-0 py-10 text-center text-text-muted-strong">
                No {lower(labels.projects)} match “{query.trim()}”.
              </p>
            )}
            <button
              type="button"
              className="flex min-h-[280px] cursor-pointer flex-col items-center justify-center gap-2.5 rounded-[14px] border border-dashed border-line-strong bg-transparent p-5 font-[inherit] text-text-muted-strong"
            >
              <Icon name="plus" size={28} />
              <span className="font-medium text-text">{labels.newProject}</span>
              <span className="text-[13px]">{labels.newProjectHint}</span>
            </button>
          </section>
        )}
      </main>
    </div>
  )
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`
const lower = (s: string) => s.toLowerCase()

function ProjectCard({ project: p }: { project: ProjectSummary }) {
  const members = p.members
  const sprint = p.activeSprint

  return (
    <Link
      to={`/projects/${p.id}`}
      className="flex flex-col gap-4 rounded-[14px] border border-line bg-surface p-5 text-text no-underline transition-[border-color,translate] hover:-translate-y-px hover:border-ion hover:text-text"
    >
      <div className="flex items-start gap-3.5">
        <span className="inline-flex size-11 flex-none items-center justify-center rounded-card border border-line bg-surface-sunken">
          <Icon name="orbit" size={26} strokeWidth={1.6} style={{ color: p.color }} />
        </span>
        <div className="min-w-0 flex-1">
          <div className="font-heading text-[19px] font-semibold">{p.name}</div>
          <div className="mt-1 text-[13px] leading-[1.45] text-text-muted-strong">{p.description}</div>
        </div>
      </div>

      <div className="flex items-center justify-between gap-2">
        {sprint ? (
          <Chip tone="ion" className="px-2.5">
            {labels.inSprint(sprint.number)}
          </Chip>
        ) : (
          <Chip tone="neutral" className="px-2.5">
            {labels.betweenSprints}
          </Chip>
        )}
        <span className="font-mono text-xs text-text-muted">
          {sprint ? `T-${countdown(sprint.end)}` : p.nextSprintStart ? labels.nextSprint(shortDate(p.nextSprintStart)) : ''}
        </span>
      </div>

      <ProgressBar label={labels.tasksLanded} value={p.stats.tasksLanded} max={p.stats.tasksTotal} />

      <div className="flex flex-wrap gap-2">
        {p.stats.awaitingApproval > 0 && (
          <Chip tone="warning" shape="tag">
            <Icon name="dock" size={14} />
            {p.stats.awaitingApproval} {lower(labels.awaitingApproval)}
          </Chip>
        )}
        {p.stats.blocked > 0 && (
          <Chip tone="critical" shape="tag">
            <Icon name="alert" size={14} />
            {p.stats.blocked} {lower(labels.blocked)}
          </Chip>
        )}
        <Chip tone="astronaut" shape="tag">
          <Icon name="helmet" size={14} className="text-astronaut" />
          {plural(p.stats.activeAgents, lower(labels.agent), lower(labels.agents))}
        </Chip>
      </div>

      <div className="flex items-center justify-between border-t border-line-soft pt-3.5">
        <div className="flex">
          {members.map((u) => (
            <UserAvatar key={u.id} user={u} stacked />
          ))}
        </div>
        <span className="text-xs text-text-muted">
          {plural(members.length, lower(labels.user), lower(labels.users))} · {timeAgo(p.lastActivityAt)}
        </span>
      </div>
    </Link>
  )
}
