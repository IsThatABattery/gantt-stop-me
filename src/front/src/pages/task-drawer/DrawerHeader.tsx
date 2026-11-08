import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Chip } from '../../components/Chip'
import { Icon } from '../../components/Icon'
import type { TaskDetail } from '../../data/api/types'
import type { ColumnKey } from '../../data/types'
import { actionLabels, columnLabels, runStatusLabels } from '../../labels/labels'
import { runLook } from '../board/runLook'

interface DrawerHeaderProps {
  projectId: string
  task: TaskDetail
  /** The task's column, once the board has loaded */
  column?: ColumnKey
}

export function DrawerHeader({ projectId, task, column }: DrawerHeaderProps) {
  const run = task.runStatus ? runLook[task.runStatus] : null
  const closed = task.state === 'closed'

  return (
    <header className="flex flex-col gap-3 border-b border-line-soft px-6 pt-5 pb-4">
      <div className="flex flex-wrap items-center gap-2.5 pr-12">
        <span className="font-mono text-[13px] text-text-muted">#{task.issueNumber}</span>
        {column && (
          <Chip tone={closed ? 'good' : 'neutral'}>
            {closed && <Icon name="check" size={12} />}
            {columnLabels[column].name} · {columnLabels[column].plain}
          </Chip>
        )}
        <Link to={`/projects/${projectId}/tasks/${task.id}/edit`} className={buttonClass('secondary', 'ml-auto px-3.5')}>
          <Icon name="edit" size={16} />
          {actionLabels.editTask}
        </Link>
      </div>
      <h1 className="m-0 font-heading text-[22px] leading-[1.3] font-semibold">{task.title}</h1>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-[13px] text-text-muted">
        <a href={task.url} target="_blank" rel="noreferrer" className="inline-flex min-h-11 items-center gap-1.5">
          <Icon name="github" size={14} />
          Issue #{task.issueNumber} on GitHub
        </a>
        <span>{closed ? 'Closed' : 'Open'}</span>
        {run && task.runStatus && (
          <Chip tone={run.tone}>
            {run.icon && <Icon name={run.icon} size={12} />}
            {runStatusLabels[task.runStatus]}
          </Chip>
        )}
      </div>
      {task.needsAttention && (
        <div className="flex items-start gap-1.5 rounded-lg border border-critical/60 bg-critical/8 px-2.5 py-2 text-[13px] text-critical">
          <Icon name="alert" size={16} />
          <span>
            <span className="sr-only">Needs attention: </span>
            {task.needsAttention}
          </span>
        </div>
      )}
    </header>
  )
}
