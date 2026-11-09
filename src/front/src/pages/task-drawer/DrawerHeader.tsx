import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Chip } from '../../components/Chip'
import { Icon } from '../../components/Icon'
import { runLook } from '../../components/runLook'
import type { Task, TaskDetail } from '../../data/api/types'
import type { ColumnKey } from '../../data/types'
import { actionLabels, labels, runStatusLabels } from '../../labels/labels'
import { columnName, personNeeded } from './state'

/**
 * The drawer's ink tab, as a stage tab on the board: the column in display caps, the issue number where the board's
 * count sits, and the signal or approve mark when the task waits on a person. The Drawer hides it from screen
 * readers; the header below says all of it in words.
 */
export function DrawerTab({ task, column }: { task: Task; column?: ColumnKey }) {
  const mark = personNeeded(task, column)
  return (
    <span className="flex min-w-0 items-center gap-2.5">
      <span className="truncate">{column ? columnName(column) : labels.task}</span>
      <span className="flex-none rounded-[2px] border border-on-ink/50 px-1.5 font-mono text-sm leading-[22px] font-semibold tracking-normal normal-case">
        #{task.issueNumber}
      </span>
      {mark && (
        <span className={`grid size-6 flex-none place-items-center rounded-[2px] ${mark === 'signal' ? 'bg-signal text-on-signal' : 'bg-approve text-on-approve'}`}>
          <Icon name={mark === 'signal' ? 'alert' : 'person'} size={14} />
        </span>
      )}
    </span>
  )
}

interface DrawerHeaderProps {
  projectId: string
  task: TaskDetail
  /** The task's column, once the board has loaded */
  column?: ColumnKey
}

/** Title, state chips, the issue link and Edit, and why the task needs attention. */
export function DrawerHeader({ projectId, task, column }: DrawerHeaderProps) {
  const run = task.runStatus ? runLook[task.runStatus] : null
  const closed = task.state === 'closed'
  const awaiting = column === 'user-check' && !closed

  return (
    <header className="flex flex-col gap-2.5 px-5 pt-5 pb-4 sm:px-6">
      {column && <p className="sr-only">In {columnName(column)}</p>}
      <div className="flex items-start gap-3">
        <h1 className="m-0 min-w-0 flex-1 pt-1.5 text-[22px] leading-[1.25] font-bold tracking-[-0.01em] text-balance break-words">{task.title}</h1>
        <Link to={`/projects/${projectId}/tasks/${task.id}/edit`} className={buttonClass('secondary', 'flex-none px-3.5')}>
          <Icon name="edit" size={16} />
          {actionLabels.editTask}
        </Link>
      </div>

      <div className="flex flex-wrap items-center gap-x-3.5 gap-y-1">
        {(awaiting || run || closed) && (
          <span className="flex flex-wrap items-center gap-1.5">
            {awaiting && (
              <Chip tone="approve" icon="person">
                {labels.awaitingApproval}
              </Chip>
            )}
            {run && task.runStatus && (
              <Chip tone={run.tone} icon={run.icon} iconClassName={run.iconClass}>
                {runStatusLabels[task.runStatus]}
              </Chip>
            )}
            {closed && (
              <Chip tone="neutral" soft icon="closed">
                Closed on GitHub
              </Chip>
            )}
          </span>
        )}
        <a href={task.url} target="_blank" rel="noreferrer" className="inline-flex min-h-11 items-center gap-1.5 text-[13px] font-semibold">
          <Icon name="github" size={16} />
          Issue #{task.issueNumber} on GitHub
        </a>
      </div>

      {task.needsAttention && (
        <div className="flex gap-2.5 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] leading-normal text-signal-ink">
          <Icon name="alert" size={16} className="mt-px flex-none" />
          <div className="flex min-w-0 flex-col gap-0.5">
            <p className="m-0 font-bold">{labels.needsAttention}</p>
            <p className="m-0 break-words">{task.needsAttention}</p>
            <p className="m-0 mt-1">The run broke, so it says nothing about the code. Retest once the cause is fixed.</p>
          </div>
        </div>
      )}
    </header>
  )
}
