import { Link } from 'react-router'
import { UserAvatar } from '../../components/Avatar'
import { Chip } from '../../components/Chip'
import { Icon } from '../../components/Icon'
import { ProgressBar } from '../../components/ProgressBar'
import type { ProjectSummary } from '../../data/types'
import { labels } from '../../labels/labels'
import { countdown, shortDate, timeAgo } from '../../lib/time'
import { plural } from './filters'

const lower = (s: string) => s.toLowerCase()

interface ProjectCardProps {
  project: ProjectSummary
  favorite: boolean
  onToggleFavorite: () => void
}

/**
 * One mission card. The star is a sibling of the card link, laid over its
 * corner, so no interactive element sits inside another.
 */
export function ProjectCard({ project: p, favorite, onToggleFavorite }: ProjectCardProps) {
  const members = p.members
  const sprint = p.activeSprint

  return (
    <div className="relative">
      <Link
        to={`/projects/${p.id}`}
        className="flex h-full flex-col gap-4 rounded-card border border-line bg-surface p-5 text-text no-underline transition-[border-color,translate] hover:-translate-y-px hover:border-ion hover:text-text"
      >
        <div className="pr-10">
          <div className="min-w-0">
            <div className="font-heading text-[19px] font-semibold">{p.name}</div>
            {p.description && <div className="mt-1 text-[13px] leading-[1.45] text-text-muted-strong">{p.description}</div>}
          </div>
        </div>

        <div className="flex items-center justify-between gap-2">
          {p.archived ? (
            <Chip tone="neutral" className="px-3">
              Archived
            </Chip>
          ) : sprint ? (
            <Chip tone="ion" className="px-3">
              {labels.inSprint(sprint.number)}
            </Chip>
          ) : (
            <Chip tone="neutral" className="px-3">
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

        <div className="mt-auto flex items-center justify-between border-t border-line-soft pt-4">
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

      <button
        type="button"
        aria-label={`Favorite ${p.name}`}
        aria-pressed={favorite}
        onClick={onToggleFavorite}
        className={`absolute top-3 right-3 inline-flex size-11 cursor-pointer items-center justify-center rounded-control border-0 bg-transparent ${
          favorite ? 'text-warning' : 'text-text-muted hover:text-text'
        }`}
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill={favorite ? 'currentColor' : 'none'} stroke="currentColor" strokeWidth="1.75" strokeLinejoin="round" aria-hidden="true">
          <path d="M12 3.5l2.6 5.3 5.9.9-4.25 4.1 1 5.8L12 16.9l-5.25 2.7 1-5.8L3.5 9.7l5.9-.9z" />
        </svg>
      </button>
    </div>
  )
}
