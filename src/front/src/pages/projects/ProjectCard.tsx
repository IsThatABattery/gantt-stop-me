import { Link } from 'react-router'
import { UserAvatar } from '../../components/Avatar'
import { Chip } from '../../components/Chip'
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
 * One project as a white card with an inset keyline, as tasks are on the board. The star is a sibling of the card
 * link, laid over its corner, so no interactive element sits inside another.
 */
export function ProjectCard({ project: p, favorite, onToggleFavorite }: ProjectCardProps) {
  const members = p.members
  const sprint = p.activeSprint

  return (
    <div className="relative">
      <Link
        to={`/projects/${p.id}`}
        className="group flex h-full flex-col rounded-card border border-line bg-paper p-[3px] text-ink no-underline transition-colors duration-150 ease-out-quint hover:border-ink hover:text-ink"
      >
        <div className="flex flex-1 flex-col gap-4 rounded-[2px] border border-keyline p-4 transition-colors duration-150 group-hover:border-ink">
          <div className="min-w-0 pr-10">
            <div className="text-lg leading-snug font-bold">{p.name}</div>
            {p.description && <div className="mt-1 text-[13px] leading-[1.45] text-ink-2">{p.description}</div>}
          </div>

          <div className="flex items-center justify-between gap-2">
            {p.archived ? (
              <Chip tone="neutral">Archived</Chip>
            ) : sprint ? (
              <Chip tone="live">{labels.inSprint(sprint.number)}</Chip>
            ) : (
              <Chip tone="neutral" soft>
                {labels.betweenSprints}
              </Chip>
            )}
            <span className="text-xs text-ink-2">
              {sprint ? `${labels.tMinus} ${countdown(sprint.end)}` : p.nextSprintStart ? labels.nextSprint(shortDate(p.nextSprintStart)) : ''}
            </span>
          </div>

          <ProgressBar label={labels.tasksLanded} value={p.stats.tasksLanded} max={p.stats.tasksTotal} />

          <div className="flex flex-wrap gap-1.5">
            {p.stats.awaitingApproval > 0 && (
              <Chip tone="approve" icon="person">
                {p.stats.awaitingApproval} {lower(labels.awaitingApproval)}
              </Chip>
            )}
            {p.stats.blocked > 0 && (
              <Chip tone="signal" icon="alert">
                {p.stats.blocked} {lower(labels.blocked)}
              </Chip>
            )}
            <Chip tone="neutral" icon="agent" shape="tag">
              {plural(p.stats.activeAgents, lower(labels.agent), lower(labels.agents))}
            </Chip>
          </div>

          <div className="mt-auto flex items-center justify-between border-t border-keyline pt-3">
            <div className="flex">
              {members.map((u) => (
                <UserAvatar key={u.id} user={u} size="sm" stacked />
              ))}
            </div>
            <span className="text-xs text-ink-2">
              {plural(members.length, lower(labels.user), lower(labels.users))} · {timeAgo(p.lastActivityAt)}
            </span>
          </div>
        </div>
      </Link>

      <button
        type="button"
        aria-label={`Favorite ${p.name}`}
        aria-pressed={favorite}
        onClick={onToggleFavorite}
        className={`absolute top-2 right-2 inline-flex size-11 cursor-pointer items-center justify-center rounded-control border-0 bg-transparent transition-colors duration-150 hover:bg-ink/8 active:bg-ink/15 ${
          favorite ? 'text-ink' : 'text-ink-2 hover:text-ink'
        }`}
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill={favorite ? 'var(--approve)' : 'none'} stroke="currentColor" strokeWidth="2" strokeLinejoin="miter" aria-hidden="true">
          <path d="M12 3.5l2.6 5.3 5.9.9-4.25 4.1 1 5.8L12 16.9l-5.25 2.7 1-5.8L3.5 9.7l5.9-.9z" />
        </svg>
      </button>
    </div>
  )
}
