import { useState, type CSSProperties } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Drawer } from '../../components/Drawer'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { Tabs } from '../../components/Tabs'
import { tileStep } from '../../components/tileStep'
import type { TaskDetail } from '../../data/api/types'
import { NotFoundError } from '../../data/errors'
import { useBoard } from '../../data/views/board'
import { useRuns } from '../../data/views/runs'
import { useTask } from '../../data/views/tasks'
import { actionLabels, labels } from '../../labels/labels'
import { ApprovalBand } from './ApprovalBand'
import { Details } from './Details'
import { DrawerHeader, DrawerTab } from './DrawerHeader'
import { useTestCount } from './state'
import { TestRuns } from './TestRuns'

// The task drawer over the board, showing the GitHub issue as the API last read it. The mockup's subtasks, log,
// files, trace and agent approval bar (TaskTabs, Trace, ApprovalBar) wait on data the API doesn't have yet
// (docs/implementation/api-gaps.md).
export function TaskDrawerPage() {
  const { projectId = '', taskId = '' } = useParams()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  // Close back to the page under the drawer: the board, or the Backlog list.
  const close = () => navigate(pathname.slice(0, pathname.lastIndexOf('/tasks/')))
  const { data: task, isPending, error } = useTask(projectId, taskId)
  const { data: board } = useBoard(projectId)
  const runs = useRuns(projectId, taskId)
  // A confirmation after approving or sending back, kept with the task it is about.
  const [notice, setNotice] = useState({ taskId: '', text: '' })
  const index = board?.columns.findIndex((c) => c.id === task?.columnId) ?? -1
  const column = index >= 0 ? board?.columns[index] : undefined

  if (isPending || error) {
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
            <Link to={`/projects/${projectId}`} className={buttonClass('secondary')}>
              {actionLabels.backToBoard}
            </Link>
          </StatusMessage>
        ) : (
          <StatusMessage tone="signal" title="Something went wrong" body={error.message} />
        )}
      </Drawer>
    )
  }

  const awaiting = column?.key === 'user-check' && task.state === 'open' && !runs.data?.[0]?.approvedAt

  return (
    <Drawer
      label={labels.task}
      title={<DrawerTab task={task} column={column?.key} />}
      onClose={close}
      footer={awaiting ? <ApprovalBand key={task.id} projectId={projectId} task={task} onDone={(text) => setNotice({ taskId, text })} /> : undefined}
    >
      <div className="flex min-h-full flex-col">
        <DrawerHeader projectId={projectId} task={task} column={column?.key} />
        <div role="status" className="px-5 sm:px-6">
          {notice.taskId === taskId && notice.text && (
            <p className="m-0 mb-4 flex items-center gap-2 rounded-[2px] bg-block px-3 py-2.5 text-[13px] font-semibold">
              <Icon name="check" size={16} className="flex-none text-good" />
              {notice.text}
            </p>
          )}
        </div>
        <TaskSections key={task.id} projectId={projectId} task={task} step={board && column ? tileStep(index, board.columns.length) : 0} />
      </div>
    </Drawer>
  )
}

interface TaskSectionsProps {
  projectId: string
  task: TaskDetail
  /** The column's place on the board, so the drawer's tile matches its stage tile */
  step: number
}

/** Details and Tests, on a tile stepped like the task's own column. A task that has been tested opens on Tests. */
function TaskSections({ projectId, task, step }: TaskSectionsProps) {
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
            content: <TestRuns projectId={projectId} task={task} />,
          },
        ]}
      />
    </div>
  )
}
