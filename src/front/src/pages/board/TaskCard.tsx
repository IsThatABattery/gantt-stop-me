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
function look(status: TaskStatus, byAgent: boolean): CardLook {
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
}

export function TaskCard({ task, epic, feature, owner }: TaskCardProps) {
  const byAgent = owner?.kind === 'agent'
  const l = look(task.status, byAgent)
  const landed = task.status === 'landed'

  return (
    <article
      tabIndex={0}
      aria-label={`${task.key}: ${task.title}. ${taskStatusLabels[task.status]}`}
      className={`relative overflow-hidden rounded-control border bg-surface text-text outline-none ${l.border ?? 'border-line'} ${
        landed
          ? 'opacity-60 hover:border-ion focus-visible:border-ion'
          : 'transition-[border-color,translate] hover:-translate-y-px hover:border-ion focus-visible:-translate-y-px focus-visible:border-ion'
      }`}
    >
      <div className="h-[3px]" style={{ background: epic?.color ?? 'var(--color-line)' }} />
      <div className="flex flex-col gap-2.5 p-3">
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
          <div className="flex items-center gap-1.5 text-xs text-text-muted-strong">
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
          <div className="flex items-start gap-1.5 rounded-lg border border-critical/60 bg-critical/8 px-2 py-1.5 text-xs text-critical">
            <span className="sr-only">Blocked because: </span>
            {task.blockedReason}
          </div>
        )}
      </div>
    </article>
  )
}
