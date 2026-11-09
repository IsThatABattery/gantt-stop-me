import { Link, Outlet, useParams } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { buttonClass } from '../../components/buttonClass'
import { Chip } from '../../components/Chip'
import { Button } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { PageTitle } from '../../components/PageTitle'
import { runLook } from '../../components/runLook'
import { Tile } from '../../components/Tile'
import type { Task } from '../../data/api/types'
import { useBoard } from '../../data/views/board'
import { labels, noTestRunLabel, runStatusLabels } from '../../labels/labels'
import { Notice } from '../projects/Notice'

const lower = (s: string) => s.toLowerCase()
const taskCount = (n: number) => `${n} ${lower(n === 1 ? labels.task : labels.tasks)}`

/** The latest test run as a chip, or a quiet line when there has been none. */
function RunState({ task }: { task: Task }) {
  if (!task.runStatus)
    return (
      <span className="inline-flex h-6 items-center gap-[5px] text-xs font-medium whitespace-nowrap text-ink-2">
        <Icon name="idle" size={14} className="flex-none" />
        {noTestRunLabel}
      </span>
    )
  const look = runLook[task.runStatus]
  return (
    <Chip tone={look.tone} icon={look.icon} iconClassName={look.iconClass}>
      {runStatusLabels[task.runStatus]}
    </Chip>
  )
}

/** Up to three assignees as small squares, with the rest counted. */
function Assignees({ task }: { task: Task }) {
  const people = task.assignees
  if (people.length === 0) return <span className="text-xs font-medium whitespace-nowrap text-ink-2">{labels.unassigned}</span>
  return (
    <span className="flex gap-[3px]" title={people.map((a) => a.login).join(', ')}>
      <span className="sr-only">Assigned to {people.map((a) => a.login).join(', ')}</span>
      {people.slice(0, 3).map((a) => (
        <img key={a.login} src={a.avatarUrl} alt="" width={22} height={22} className="block size-[22px] rounded-[2px] bg-block" />
      ))}
      {people.length > 3 && (
        <span aria-hidden="true" className="grid h-[22px] min-w-[22px] place-items-center rounded-[2px] bg-block px-1 text-[11px] font-bold">
          +{people.length - 3}
        </span>
      )}
    </span>
  )
}

/**
 * One task as a row: a white card with an inset keyline, as on the board, laid out across. The edge and keyline close
 * in ink under the pointer; a task that needs attention keeps its signal edge and shows why.
 */
function TaskRow({ task }: { task: Task }) {
  const trouble = !!task.needsAttention
  return (
    <Link
      to={`tasks/${task.id}`}
      className={`group block rounded-card border bg-paper p-[3px] text-ink no-underline transition-colors duration-150 ease-out-quint hover:border-ink hover:text-ink active:bg-block ${
        trouble ? 'border-signal' : 'border-line'
      }`}
    >
      <span
        className={`grid grid-cols-[3.5rem_minmax(0,1fr)_auto] items-center gap-x-4 gap-y-2 rounded-[2px] border py-2.5 pr-2 pl-3 transition-colors duration-150 sm:grid-cols-[3.5rem_minmax(0,1fr)_9.5rem_7rem_1.25rem] ${
          trouble ? 'border-signal group-hover:border-signal-ink' : 'border-keyline group-hover:border-ink'
        }`}
      >
        <span className="col-start-1 row-start-1 font-mono text-[12.5px] font-semibold text-ink-2">#{task.issueNumber}</span>
        <span className="col-[1/-1] row-start-2 flex min-w-0 flex-col gap-1.5 sm:col-[2] sm:row-start-1">
          <span className="text-sm leading-snug font-semibold">{task.title}</span>
          {task.branch && (
            <span className="flex min-w-0 items-center gap-1 text-xs text-ink-2">
              <Icon name="pr" size={12} className="flex-none" />
              <span className="truncate font-mono">{task.branch}</span>
            </span>
          )}
          {trouble && (
            <span className="rounded-[2px] bg-signal-wash px-2 py-1.5 text-[12.5px] leading-snug text-signal-ink">
              <span className="sr-only">Needs attention: </span>
              {task.needsAttention}
            </span>
          )}
        </span>
        <span className="col-[1/-1] row-start-3 flex sm:col-[3] sm:row-start-1">
          <RunState task={task} />
        </span>
        <span className="col-[3] row-start-1 flex justify-end sm:col-[4] sm:justify-start">
          <Assignees task={task} />
        </span>
        <Icon name="chevronRight" size={18} className="hidden flex-none text-ink-2 group-hover:text-ink sm:col-[5] sm:row-start-1 sm:block" />
      </span>
    </Link>
  )
}

/** Placeholder rows while the board loads, in the rows' own shape. */
function LoadingRows() {
  return (
    <div role="status" className="flex flex-col gap-2">
      <span className="sr-only">Loading the {labels.backlog}…</span>
      {[0, 1, 2, 3].map((i) => (
        <span key={i} aria-hidden="true" className="h-[58px] rounded-card border border-tile-keyline bg-paper/55" />
      ))}
    </div>
  )
}

/** The Backlog page: the Backlog column as a list, in board order, on its stage tile. The drawer opens over it. */
export function HangarPage() {
  const project = useProjectContext()
  const { projectId = '' } = useParams()
  const { data: board, isPending, error, refetch } = useBoard(projectId)
  const tasks = board?.columns.find((c) => c.key === 'backlog')?.tasks ?? []

  return (
    <>
      <div className="flex min-w-0 flex-1 flex-col gap-6 px-4 pt-5 pb-10 md:px-7">
        <header className="flex flex-wrap items-end justify-between gap-x-8 gap-y-4">
          <div className="min-w-0">
            <PageTitle>{labels.backlog}</PageTitle>
            <p className="mt-2.5 mb-0 text-[15px] text-ink-2" aria-live="polite">
              <span className="font-semibold text-ink">
                {project.owner}/{project.name}
              </span>
              {board && (
                <>
                  <span aria-hidden="true"> · </span>
                  {tasks.length === 0 ? `No ${lower(labels.tasks)} waiting to start` : `${taskCount(tasks.length)} waiting to start, in board order`}
                </>
              )}
            </p>
          </div>
          <Link to={`/projects/${projectId}`} className={buttonClass('secondary', 'min-h-[52px] px-4')}>
            <Icon name="board" size={18} />
            Open the {lower(labels.board)}
          </Link>
        </header>

        {error ? (
          <Notice
            role="alert"
            tone="signal"
            title={`The ${labels.backlog} didn't load`}
            actions={
              <Button variant="primary" onClick={() => refetch()}>
                Try again
              </Button>
            }
          >
            {error.message}
          </Notice>
        ) : (
          <Tile step={0} className="max-w-[1080px]" frameClassName="p-1.5 sm:p-2">
            {isPending ? (
              <LoadingRows />
            ) : tasks.length === 0 ? (
              <div role="status" className="flex flex-col items-center gap-1.5 rounded-[3px] border border-line px-4 pt-7 pb-6 text-center">
                <Icon name="backlog" size={22} className="mb-1 text-ink-2" />
                <p className="m-0 text-sm font-bold">The {labels.backlog} is empty.</p>
                <p className="m-0 max-w-[44ch] text-[13px] leading-normal text-ink-2">
                  New {lower(labels.tasks)} wait here until someone starts them. Add one with {labels.newTask} on the {lower(labels.board)}.
                </p>
              </div>
            ) : (
              <ul aria-label={`${labels.backlog} ${lower(labels.tasks)}`} className="m-0 flex list-none flex-col gap-2 p-0">
                {tasks.map((t) => (
                  <li key={t.id}>
                    <TaskRow task={t} />
                  </li>
                ))}
              </ul>
            )}
          </Tile>
        )}
      </div>
      <Outlet context={project} />
    </>
  )
}
