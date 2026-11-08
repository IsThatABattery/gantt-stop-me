import { Link, useNavigate, useParams } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Drawer } from '../../components/Drawer'
import { StatusMessage } from '../../components/StatusMessage'
import { NotFoundError } from '../../data/errors'
import { useBoard } from '../../data/views/board'
import { useTask } from '../../data/views/tasks'
import { actionLabels, formLabels, labels } from '../../labels/labels'
import { DrawerHeader } from './DrawerHeader'

// Payload drawer over the board (PayloadDetail.dc.html), showing the GitHub
// issue as the API last read it. The mockup's modules, flight log, cargo,
// trace and approval bar need data the API doesn't have yet; their
// components stay in this folder (docs/implementation/api-gaps.md).
export function TaskDrawerPage() {
  const { projectId = '', taskId = '' } = useParams()
  const navigate = useNavigate()
  const close = () => navigate(`/projects/${projectId}`)
  const { data: task, isPending, error } = useTask(projectId, taskId)
  const { data: board } = useBoard(projectId)
  const column = board?.columns.find((c) => c.id === task?.columnId)

  if (isPending || error) {
    return (
      <Drawer label={labels.task} onClose={close}>
        {isPending ? (
          <StatusMessage title={`Loading ${labels.task.toLowerCase()}…`} />
        ) : error instanceof NotFoundError ? (
          <StatusMessage
            title={`${labels.task} not found`}
            body={`There's no ${labels.task.toLowerCase()} with that id in this ${labels.project.toLowerCase()}. It may have been deleted, or the link is wrong.`}
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

  // board:* labels repeat the column, which the header already shows.
  const shown = task.labels.filter((l) => !l.name.startsWith('board:'))

  return (
    <Drawer label={labels.task} onClose={close}>
      <div className="h-[3px] bg-ion" />
      <DrawerHeader projectId={projectId} task={task} column={column?.key} />
      <div className="flex flex-col gap-4 px-6 py-5">
        {shown.length > 0 && (
          <section className="flex flex-col gap-2">
            <h2 className="eyebrow m-0">Labels</h2>
            <ul className="m-0 flex list-none flex-wrap gap-2 p-0">
              {shown.map((l) => (
                <li key={l.name} className="inline-flex items-center gap-1.5 rounded-lg border border-line px-2.5 py-[3px] text-xs text-text-soft">
                  <span className="size-2.5 flex-none rounded-full" style={{ background: `#${l.color}` }} aria-hidden="true" />
                  {l.name}
                </li>
              ))}
            </ul>
          </section>
        )}
        <section className="flex flex-col gap-2">
          <h2 className="eyebrow m-0">{formLabels.description}</h2>
          {task.body.trim() ? (
            // Issue bodies are Markdown; shown as written until there's a renderer.
            <p className="m-0 leading-normal break-words whitespace-pre-wrap text-text-soft">{task.body}</p>
          ) : (
            <p className="m-0 text-[13px] leading-normal text-text-muted">No description yet.</p>
          )}
        </section>
      </div>
    </Drawer>
  )
}
