import { useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Drawer } from '../../components/Drawer'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { NotFoundError } from '../../data/core'
import { needsApproval, useTaskDetail, type TaskDetail } from '../../data/views/taskDrawer'
import { actionLabels, labels } from '../../labels/labels'
import { ApprovalBar } from './ApprovalBar'
import { DrawerHeader } from './DrawerHeader'
import { TaskTabs } from './TaskTabs'
import { Trace } from './Trace'

// Payload drawer over the board or the Hangar list (PayloadDetail.dc.html, AGENT_BRIEF §5–6).
export function TaskDrawerPage() {
  const { projectId = '', taskId = '' } = useParams()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  // Close back to the page under the drawer: the board, or the Hangar list.
  const close = () => navigate(pathname.slice(0, pathname.lastIndexOf('/tasks/')))
  const { data: detail, isPending, error } = useTaskDetail(projectId, taskId)
  // Keyed by task so switching cards clears the last confirmation.
  const [notice, setNotice] = useState<{ taskId: string; text: string } | null>(null)

  if (isPending || error) {
    return (
      <Drawer label={labels.task} onClose={close}>
        {isPending ? (
          <StatusMessage title={`Loading ${labels.task.toLowerCase()}…`} />
        ) : error instanceof NotFoundError ? (
          <StatusMessage
            title={`${labels.task} not found`}
            body={`There's no ${labels.task.toLowerCase()} called “${taskId}” in this ${labels.project.toLowerCase()}. It may have been deleted, or the link is wrong.`}
          >
            <Link to={`/projects/${projectId}`} className={buttonClass('secondary')}>
              {actionLabels.backToBoard}
            </Link>
          </StatusMessage>
        ) : (
          <StatusMessage title="Something went wrong" body={error.message} />
        )}
      </Drawer>
    )
  }

  return (
    <Drawer
      label={labels.task}
      onClose={close}
      footer={needsApproval(detail.task) ? <ApprovalBar key={detail.task.id} detail={detail} onDone={(text) => setNotice({ taskId, text })} /> : undefined}
    >
      <DrawerBody detail={detail} notice={notice?.taskId === taskId ? notice.text : null} />
    </Drawer>
  )
}

function DrawerBody({ detail, notice }: { detail: TaskDetail; notice: string | null }) {
  const showTrace = detail.assignee?.kind === 'agent' || detail.trace.length > 0
  return (
    <>
      <div className="h-[3px]" style={{ background: detail.epic?.color ?? 'var(--color-ion)' }} />
      <DrawerHeader detail={detail} />
      <div role="status" aria-live="polite">
        {notice && (
          <p className="m-0 flex items-center gap-2 border-b border-good/40 bg-good/8 px-6 py-3 text-[13px] text-good">
            <Icon name="check" size={16} />
            {notice}
          </p>
        )}
      </div>
      <TaskTabs key={detail.task.id} detail={detail} />
      {showTrace &&
        (detail.trace.length > 0 ? (
          <Trace steps={detail.trace} />
        ) : (
          <p className="m-0 border-t border-line-soft px-6 py-5 text-[13px] text-text-muted">
            No {labels.agent.toLowerCase()} trace yet.
          </p>
        ))}
    </>
  )
}
