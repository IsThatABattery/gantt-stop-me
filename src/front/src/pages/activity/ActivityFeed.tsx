import { useState } from 'react'
import { Link } from 'react-router'
import { Chip } from '../../components/Chip'
import { Icon } from '../../components/Icon'
import type { ActivityEvent, Agent, Assignee, Task, User } from '../../data/types'
import { activityVerbLabels, labels } from '../../labels/labels'
import { clockTime, filterEvents, filterLabels, groupByDay, type ActivityFilter } from './feed'

/** Toggle buttons for the four filters, with a count on each. */
export function ActivityFilters({
  events,
  value,
  onChange,
}: {
  events: ActivityEvent[]
  value: ActivityFilter
  onChange: (f: ActivityFilter) => void
}) {
  return (
    <div role="group" aria-label={`Filter the ${labels.activity.toLowerCase()}`} className="flex flex-wrap gap-2">
      {(Object.keys(filterLabels) as ActivityFilter[]).map((f) => {
        const on = f === value
        return (
          <button
            key={f}
            type="button"
            aria-pressed={on}
            onClick={() => onChange(f)}
            className={`inline-flex min-h-11 cursor-pointer items-center gap-2 rounded-control border px-3.5 font-[inherit] ${
              on ? 'border-ion bg-ion/10 text-text' : 'border-line bg-transparent text-text-muted-strong hover:border-line-strong hover:text-text'
            }`}
          >
            {f === 'needs-action' && <Icon name="alert" size={14} className="text-warning" />}
            {filterLabels[f]}
            <span className="font-mono text-xs text-text-muted">{filterEvents(events, f).length}</span>
          </button>
        )
      })}
    </div>
  )
}

interface ActivityFeedProps {
  events: ActivityEvent[]
  users: User[]
  agents: Agent[]
  tasks: Pick<Task, 'id' | 'key' | 'title'>[]
  /** Shown when there are no events */
  emptyText?: string
  now?: number
}

/**
 * The Mission log entries, newest first, grouped by day once there's more
 * than one day. Works from a ~360px drawer up to the full page.
 */
export function ActivityFeed({ events, users, agents, tasks, emptyText = 'No activity yet.', now }: ActivityFeedProps) {
  const [today] = useState(() => now ?? Date.now())
  if (events.length === 0) {
    return (
      <p role="status" className="m-0 rounded-card border border-dashed border-line px-4 py-10 text-center text-[13px] text-text-muted">
        {emptyText}
      </p>
    )
  }

  const groups = groupByDay(events, today)
  const lookup = {
    user: new Map(users.map((u) => [u.id, u])),
    agent: new Map(agents.map((a) => [a.id, a])),
    task: new Map(tasks.map((t) => [t.id, t])),
  }

  const list = (items: ActivityEvent[]) => (
    <ol className="m-0 flex list-none flex-col gap-2 p-0">
      {items.map((e) => (
        <Entry key={e.id} event={e} lookup={lookup} />
      ))}
    </ol>
  )

  if (groups.length === 1) return list(groups[0].events)
  return (
    <div className="flex flex-col gap-5">
      {groups.map((g) => (
        <section key={g.key} aria-label={g.label} className="flex flex-col gap-2">
          <h2 className="eyebrow m-0">{g.label}</h2>
          {list(g.events)}
        </section>
      ))}
    </div>
  )
}

interface Lookup {
  user: Map<string, User>
  agent: Map<string, Agent>
  task: Map<string, Pick<Task, 'id' | 'key' | 'title'>>
}

function Entry({ event: e, lookup }: { event: ActivityEvent; lookup: Lookup }) {
  const task = e.taskId ? lookup.task.get(e.taskId) : undefined
  return (
    <li
      className={`flex gap-3 rounded-control border px-3.5 py-3 text-[13px] leading-normal ${
        e.needsAction ? 'border-warning/70 bg-warning/6' : 'border-line-soft bg-surface'
      }`}
    >
      <time dateTime={e.at} className="w-11 flex-none pt-0.5 font-mono text-xs text-text-muted">
        {clockTime(e.at)}
      </time>
      <ActorMark actor={e.actor} lookup={lookup} />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p className="m-0 break-words">
          <ActorName actor={e.actor} lookup={lookup} /> {activityVerbLabels[e.verb]}{' '}
          {task ? (
            <Link to={`/projects/${e.projectId}/tasks/${task.id}`} title={task.title} className="font-mono text-xs">
              {task.key}
            </Link>
          ) : (
            e.taskId && <span className="font-mono text-xs text-text-muted">{e.taskId}</span>
          )}
        </p>
        {e.detail && <p className="m-0 break-words text-text-muted-strong">{e.detail}</p>}
        {e.needsAction && (
          <Chip tone="warning" className="mt-0.5 self-start">
            <Icon name="alert" size={12} />
            {labels.needsAction}
          </Chip>
        )}
      </div>
    </li>
  )
}

/** Helmet for an astronaut, initials for an operator. Decoration: the name says who. */
function ActorMark({ actor, lookup }: { actor: Assignee; lookup: Lookup }) {
  if (actor.kind === 'agent') {
    return (
      <span aria-hidden="true" className="inline-flex size-7 flex-none items-center justify-center rounded-lg border border-astronaut bg-astronaut/10 text-astronaut">
        <Icon name="helmet" size={16} />
      </span>
    )
  }
  return (
    <span aria-hidden="true" className="inline-flex size-7 flex-none items-center justify-center rounded-full bg-line text-[11px] font-semibold text-text">
      {lookup.user.get(actor.id)?.initials ?? '?'}
    </span>
  )
}

/** Astronaut names violet, operator names cyan (AGENT_BRIEF §5). */
function ActorName({ actor, lookup }: { actor: Assignee; lookup: Lookup }) {
  if (actor.kind === 'agent') {
    return (
      <span className="font-medium text-astronaut-text">
        <span className="sr-only">{labels.agent} </span>
        {lookup.agent.get(actor.id)?.name ?? labels.agent}
      </span>
    )
  }
  return (
    <span className="font-medium text-ion">
      <span className="sr-only">{labels.user} </span>
      {lookup.user.get(actor.id)?.name ?? labels.user}
    </span>
  )
}
