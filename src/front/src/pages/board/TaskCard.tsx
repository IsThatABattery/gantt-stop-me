import { Link } from 'react-router'
import { Chip } from '../../components/Chip'
import { Icon } from '../../components/Icon'
import type { Task } from '../../data/api/types'
import { runStatusLabels, taskStatusLabels } from '../../labels/labels'
import { runLook } from '../../components/runLook'

interface TaskCardProps {
  task: Task
  /** Opens the payload drawer */
  to: string
  /** The copy under the pointer while dragging: not focusable, hidden from screen readers */
  overlay?: boolean
}

/** One GitHub issue on the board: its number, title, test status and any runner trouble. */
export function TaskCard({ task, to, overlay = false }: TaskCardProps) {
  const closed = task.state === 'closed'
  const run = task.runStatus ? runLook[task.runStatus] : null
  const status = [closed && taskStatusLabels.landed, task.runStatus && runStatusLabels[task.runStatus]].filter(Boolean).join(', ')

  return (
    <Link
      to={to}
      data-task-id={overlay ? undefined : task.id}
      tabIndex={overlay ? -1 : undefined}
      aria-hidden={overlay || undefined}
      aria-describedby={overlay ? undefined : 'board-keys'}
      // The board's own drag replaces the browser's native link drag.
      draggable={false}
      aria-label={`#${task.issueNumber}: ${task.title}${status ? `. ${status}` : ''}${task.assignees.length ? `. Assigned to ${task.assignees.map((a) => a.login).join(', ')}` : ''}`}
      className={`relative block overflow-hidden rounded-control border bg-surface text-text no-underline outline-none hover:text-text ${
        task.needsAttention ? 'border-critical' : 'border-line'
      } ${overlay ? 'shadow-xl shadow-black/40' : ''} ${
        closed
          ? 'opacity-60 hover:border-ion focus-visible:border-ion'
          : 'transition-[border-color,translate] hover:-translate-y-px hover:border-ion focus-visible:-translate-y-px focus-visible:border-ion'
      }`}
    >
      <div className="flex flex-col gap-2.5 p-3">
        <div className="flex items-center justify-between gap-2">
          <span className="font-mono text-xs text-text-muted">#{task.issueNumber}</span>
          {task.assignees.length > 0 && (
            <span className="flex -space-x-1.5" title={task.assignees.map((a) => a.login).join(', ')}>
              {task.assignees.slice(0, 3).map((a) => (
                <img key={a.login} src={a.avatarUrl} alt="" className="size-5 rounded-full border border-surface" />
              ))}
            </span>
          )}
        </div>
        <div className="line-clamp-2 text-sm leading-[1.4] font-medium">{task.title}</div>
        {/* Landed says it all on the card; the drawer keeps the last run. */}
        {closed ? (
          <div className="flex">
            <Chip tone="good">
              <Icon name="check" size={12} />
              {taskStatusLabels.landed}
            </Chip>
          </div>
        ) : (
          run &&
          task.runStatus && (
            <div className="flex">
              <Chip tone={run.tone}>
                {run.icon && <Icon name={run.icon} size={14} className={run.iconClass} />}
                {runStatusLabels[task.runStatus]}
              </Chip>
            </div>
          )
        )}
        {task.needsAttention && (
          <div className="flex items-start gap-1.5 rounded-lg border border-critical/60 bg-critical/8 px-2 py-1.5 text-xs text-critical">
            <span className="sr-only">Needs attention: </span>
            {task.needsAttention}
          </div>
        )}
      </div>
    </Link>
  )
}
