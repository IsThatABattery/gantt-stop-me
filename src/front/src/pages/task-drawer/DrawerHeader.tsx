import { Link } from 'react-router'
import { UserAvatar, AutonomyRings } from '../../components/Avatar'
import { buttonClass } from '../../components/buttonClass'
import { Chip } from '../../components/Chip'
import { Icon } from '../../components/Icon'
import type { TaskDetail } from '../../data/views/taskDrawer'
import { actionLabels, autonomyLabels, columnLabels, labels } from '../../labels/labels'
import { plainStatus, statusLook } from './look'

export function DrawerHeader({ detail }: { detail: TaskDetail }) {
  const { task, assignee, epic, feature, sprint } = detail
  const look = statusLook(task.status, assignee?.kind === 'agent')

  return (
    <header className="flex flex-col gap-3 border-b border-line-soft px-6 pt-5 pb-4">
      <div className="flex flex-wrap items-center gap-3 pr-12">
        <span className="font-mono text-[13px] text-text-muted">{task.key}</span>
        <Chip tone={look.tone}>
          {look.icon && <Icon name={look.icon} size={12} />}
          {columnLabels[task.column].name} · {plainStatus[task.status]}
        </Chip>
        <Link to={`/projects/${task.projectId}/tasks/${task.id}/edit`} className={buttonClass('secondary', 'ml-auto px-4')}>
          <Icon name="edit" size={16} />
          {actionLabels.editTask}
        </Link>
      </div>
      <h1 className="m-0 font-heading text-[22px] leading-[1.3] font-semibold">{task.title}</h1>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-[13px] text-text-muted">
        <Owner detail={detail} />
        <span>
          {labels.epic}: {epic?.name ?? '—'}
        </span>
        <span>
          {labels.feature}: {feature?.name ?? '—'}
        </span>
        <span>Estimate: {task.estimate} pt</span>
        <span>{sprint ? labels.sprintName(sprint.number) : labels.notScheduled}</span>
      </div>
      {task.status === 'blocked' && task.blockedReason && (
        <div className="flex items-start gap-2 rounded-control border border-critical/60 bg-critical/8 px-3 py-2 text-[13px] text-critical">
          <Icon name="alert" size={16} />
          <span>
            <span className="sr-only">Blocked because: </span>
            {task.blockedReason}
          </span>
        </div>
      )}
    </header>
  )
}

function Owner({ detail }: { detail: TaskDetail }) {
  const { assignee, task } = detail
  if (!assignee) return <span>{labels.unassigned}</span>
  if (assignee.kind === 'user') {
    return (
      <span className="inline-flex items-center gap-2 text-text-soft">
        <UserAvatar user={assignee.user} size="sm" />
        {assignee.user.name}
      </span>
    )
  }
  const autonomy = task.autonomyOverride ?? assignee.agent.autonomy
  return (
    <span className="inline-flex h-7 items-center gap-2 rounded-control border border-astronaut pr-3 pl-2 text-astronaut-text">
      <Icon name="helmet" size={16} className="text-astronaut" />
      <span className="sr-only">{labels.agent} </span>
      {assignee.agent.name} · {autonomyLabels[autonomy]}
      <AutonomyRings level={autonomy} size={12} />
    </span>
  )
}
