import { Link } from 'react-router'
import { AgentTag, UserAvatar } from '../../components/Avatar'
import { Chip, type Tone } from '../../components/Chip'
import { Icon, type IconName } from '../../components/Icon'
import type { Agent, Epic, Feature, Task, TaskStatus, User } from '../../data/types'
import { labels, taskStatusLabels } from '../../labels/labels'

interface CardLook {
  tone: Tone
  icon?: IconName
  /** Border color class when the state calls for one */
  border?: string
}

// Card states from AGENT_BRIEF §5. Every color pairs with a word and,
// where it matters, an icon.
export function cardLook(status: TaskStatus, byAgent: boolean): CardLook {
  switch (status) {
    case 'queued':
      return { tone: 'neutral' }
    case 'ready':
      return { tone: 'ion' }
    case 'working':
      return byAgent ? { tone: 'astronaut', border: 'border-astronaut' } : { tone: 'ion' }
    case 'hail':
      return { tone: 'warning', icon: 'chat', border: 'border-warning' }
    case 'blocked':
      return { tone: 'critical', icon: 'alert', border: 'border-critical' }
    case 'awaiting-approval':
      return { tone: 'warning', icon: 'dock', border: 'border-warning' }
    case 'in-review':
      return { tone: 'warning', icon: 'dock' }
    case 'landed':
      return { tone: 'good', icon: 'check' }
  }
}

/** PR number, then module progress, then the estimate. */
function progressText(t: Task) {
  if (t.prNumber) return `PR #${t.prNumber}`
  if (t.subtasks.total > 0) return `${t.subtasks.done}/${t.subtasks.total}`
  return `${t.estimate} pt`
}

interface TaskCardProps {
  task: Task
  epic?: Epic
  feature?: Feature
  owner: { kind: 'user'; user: User } | { kind: 'agent'; agent: Agent } | null
  /** Opens the payload drawer */
  to: string
  /** The copy under the pointer while dragging: not focusable, hidden from screen readers */
  overlay?: boolean
}

export function TaskCard({ task, epic, feature, owner, to, overlay = false }: TaskCardProps) {
  const byAgent = owner?.kind === 'agent'
  const l = cardLook(task.status, byAgent)
  const landed = task.status === 'landed'

  return (
    <Link
      to={to}
      data-task-id={overlay ? undefined : task.id}
      tabIndex={overlay ? -1 : undefined}
      aria-hidden={overlay || undefined}
      aria-describedby={overlay ? undefined : 'board-keys'}
      // The board's own drag replaces the browser's native link drag.
      draggable={false}
      aria-label={`${task.key}: ${task.title}. ${taskStatusLabels[task.status]}`}
      className={`relative block overflow-hidden rounded-control border bg-surface text-text no-underline outline-none hover:text-text ${l.border ?? 'border-line'} ${overlay ? 'shadow-xl shadow-black/40' : ''} ${
        landed
          ? 'opacity-60 hover:border-ion focus-visible:border-ion'
          : 'transition-[border-color,translate] hover:-translate-y-px hover:border-ion focus-visible:-translate-y-px focus-visible:border-ion'
      }`}
    >
      <div className="flex flex-col gap-3 p-3">
        <div className="flex items-center justify-between">
          <span className="font-mono text-xs text-text-muted">{task.key}</span>
          {epic && (
            <span className="text-xs text-text-muted">
              <span className="sr-only">{labels.epic}: </span>
              {epic.name}
            </span>
          )}
        </div>
        <div className="line-clamp-2 text-sm leading-[1.4] font-medium">{task.title}</div>
        {feature && (
          <div className="flex items-center gap-2 text-xs text-text-muted-strong">
            <Icon name="target" size={14} />
            <span className="sr-only">{labels.feature}: </span>
            {feature.name}
          </div>
        )}
        <div className="flex flex-wrap items-center gap-2">
          {owner?.kind === 'agent' && (
            <AgentTag agent={owner.agent} active={task.status === 'working'} hail={task.status === 'hail'} />
          )}
          {owner?.kind === 'user' && <UserAvatar user={owner.user} size="sm" />}
          <span className="font-mono text-xs text-text-muted">{progressText(task)}</span>
          <Chip tone={l.tone} className="ml-auto">
            {l.icon && <Icon name={l.icon} size={12} />}
            {taskStatusLabels[task.status]}
          </Chip>
        </div>
        {task.status === 'blocked' && task.blockedReason && (
          <div className="flex items-start gap-2 rounded-control border border-critical/60 bg-critical/8 px-2 py-2 text-xs text-critical">
            <span className="sr-only">Blocked because: </span>
            {task.blockedReason}
          </div>
        )}
      </div>
    </Link>
  )
}
