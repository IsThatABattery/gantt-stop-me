import { useState, type CSSProperties } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Button } from '../../components/controls'
import { Drawer } from '../../components/Drawer'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { Tabs } from '../../components/Tabs'
import { tileStep } from '../../components/tileStep'
import type { TaskDetail } from '../../data/api/types'
import { NotFoundError } from '../../data/errors'
import { useBoard } from '../../data/views/board'
import { useApprove, useRequestChanges, useRuns } from '../../data/views/runs'
import { useTask } from '../../data/views/tasks'
import { actionLabels, columnLabels, labels } from '../../labels/labels'
import { ApprovalBand } from './ApprovalBand'
import { Details } from './Details'
import { DrawerHeader, DrawerTab } from './DrawerHeader'
import { useTestCount } from './state'
import { TestRuns } from './TestRuns'

// The task drawer, over the board or the Backlog list, showing the GitHub issue as the API last read it.
export function TaskDrawerPage() {
  const { projectId = '', taskId = '' } = useParams()
  // One drawer per task, so an approval's confirmation or failure never carries over to the next task opened.
  return <TaskDrawer key={`${projectId}/${taskId}`} projectId={projectId} taskId={taskId} />
}

function TaskDrawer({ projectId, taskId }: { projectId: string; taskId: string }) {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  // Close back to the page under the drawer: the board, or the Backlog list.
  const under = pathname.slice(0, pathname.lastIndexOf('/tasks/'))
  const close = () => navigate(under)
  const { data: task, isPending, error, refetch } = useTask(projectId, taskId)
  const { data: board } = useBoard(projectId)
  const runs = useRuns(projectId, taskId)
  // Here, not in the band: a failed approve refetches the task, which can take it out of User Check and the band with
  // it, and its reason must still show.
  const approve = useApprove(projectId, taskId)
  const changes = useRequestChanges(projectId, taskId)
  const index = board?.columns.findIndex((c) => c.id === task?.columnId) ?? -1
  const column = index >= 0 ? board?.columns[index] : undefined
  const readOnly = board?.project.permission === 'read'

  // A failed refetch keeps the task that already loaded.
  if (!task) {
    return (
      <Drawer label={labels.task} onClose={close}>
        {isPending ? (
          <StatusMessage title={`Loading ${labels.task.toLowerCase()}…`} />
        ) : error instanceof NotFoundError ? (
          <StatusMessage
            title={`${labels.task} not found`}
            icon="search"
            body={`There's no ${labels.task.toLowerCase()} with that id in this ${labels.project.toLowerCase()}. It may have been deleted, or the link is wrong.`}
          >
            <Link to={under} className={buttonClass('secondary')}>
              {under.endsWith('/backlog') ? `Back to ${labels.backlog}` : actionLabels.backToBoard}
            </Link>
          </StatusMessage>
        ) : (
          <StatusMessage tone="signal" title="Something went wrong" body={error?.message}>
            <Button onClick={() => refetch()}>Try again</Button>
          </StatusMessage>
        )}
      </Drawer>
    )
  }

  // Read-only members get no band: approving needs write access.
  const awaiting = !readOnly && column?.key === 'user-check' && task.state === 'open' && !runs.data?.[0]?.approvedAt
  // While the band is up it shows its own errors; once the task leaves User Check the drawer keeps the last one.
  const failed = awaiting ? null : (approve.error ?? changes.error)
  const done = approve.isSuccess
    ? `Approved and merged. #${task.issueNumber} is closed.`
    : changes.isSuccess
      ? `Changes requested. #${task.issueNumber} is back in ${columnLabels['in-progress'].name}.`
      : null

  return (
    <Drawer
      label={labels.task}
      title={<DrawerTab task={task} column={column?.key} />}
      onClose={close}
      footer={awaiting ? <ApprovalBand key={task.id} projectId={projectId} task={task} approve={approve} changes={changes} /> : undefined}
    >
      <div className="flex min-h-full flex-col">
        <DrawerHeader projectId={projectId} task={task} column={column?.key} readOnly={readOnly} />
        <div role="status" className="px-5 sm:px-6">
          {done && (
            <p className="m-0 mb-4 flex items-center gap-2 rounded-[2px] bg-block px-3 py-2.5 text-[13px] font-semibold">
              <Icon name="check" size={16} className="flex-none text-good" />
              {done}
            </p>
          )}
        </div>
        {failed && (
          <p
            role="alert"
            className="mx-5 mt-0 mb-4 flex items-start gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] leading-normal text-signal-ink sm:mx-6"
          >
            <Icon name="alert" size={16} className="mt-px flex-none" />
            <span className="min-w-0 wrap-anywhere">{failed.message}</span>
          </p>
        )}
        <TaskSections
          key={task.id}
          projectId={projectId}
          task={task}
          readOnly={readOnly}
          step={board && column ? tileStep(index, board.columns.length) : 0}
        />
      </div>
    </Drawer>
  )
}

interface TaskSectionsProps {
  projectId: string
  task: TaskDetail
  readOnly: boolean
  /** The column's place on the board, so the drawer's tile matches its stage tile */
  step: number
}

/** Details and Tests, on a tile stepped like the task's own column. A task that has been tested opens on Tests. */
function TaskSections({ projectId, task, readOnly, step }: TaskSectionsProps) {
  const [tab, setTab] = useState(() => (task.runStatus || task.needsAttention ? 'tests' : 'details'))
  const count = useTestCount(projectId, task)

  return (
    <div className="tile flex-1 px-3 pt-3 pb-8 sm:px-4" style={{ '--tile-step': step } as CSSProperties}>
      <Tabs
        label={`${labels.task} sections`}
        value={tab}
        onChange={setTab}
        items={[
          { id: 'details', label: 'Details', content: <Details projectId={projectId} task={task} /> },
          {
            id: 'tests',
            label: (
              <span className="inline-flex items-center gap-2">
                {'Tests '}
                {count > 0 && <span className="min-w-5 rounded-[2px] border border-current/45 px-1 text-center text-xs leading-[18px]">{count}</span>}
              </span>
            ),
            content: <TestRuns projectId={projectId} task={task} readOnly={readOnly} />,
          },
        ]}
      />
    </div>
  )
}
