import { Link, Outlet, useParams } from 'react-router'
import { StatusMessage } from '../../components/StatusMessage'
import { Button } from '../../components/controls'
import { useBoard } from '../../data/views/board'
import { labels, runStatusLabels } from '../../labels/labels'
import { useProjectContext } from '../../app/projectContext'

const lower = (s: string) => s.toLowerCase()

/** The Hangar tab: the Hangar (Backlog) column as a list, in board order. The drawer opens over it. */
export function HangarPage() {
  const { projectId = '' } = useParams()
  const { data: board, isPending, error, refetch } = useBoard(projectId)
  const tasks = board?.columns.find((c) => c.key === 'backlog')?.tasks ?? []

  return (
    <>
      <div className="flex min-w-0 flex-1 flex-col gap-6 px-8 py-7">
        <header>
          <h1 className="m-0 font-heading text-[26px] font-semibold">{labels.backlog}</h1>
        </header>
        {isPending ? (
          <StatusMessage title={`Loading ${lower(labels.backlog)}…`} />
        ) : error && !board ? (
          <StatusMessage title="Something went wrong" body={error.message}>
            <Button onClick={() => refetch()}>Try again</Button>
          </StatusMessage>
        ) : tasks.length === 0 ? (
          <p role="status" className="m-0 py-10 text-center text-text-muted-strong">
            The {labels.backlog} is empty. New {lower(labels.tasks)} wait here until they launch.
          </p>
        ) : (
          <>
            <p className="m-0 text-sm text-text-muted">
              {tasks.length} {lower(tasks.length === 1 ? labels.task : labels.tasks)}
            </p>
            <ul aria-label={`${labels.backlog} ${lower(labels.tasks)}`} className="m-0 flex max-w-4xl list-none flex-col gap-1.5 p-0">
              {tasks.map((t) => (
                <li key={t.id}>
                  <Link
                    to={`tasks/${t.id}`}
                    className="flex min-h-12 items-center gap-3 rounded-lg border border-line-strong bg-surface-sunken px-4 text-text no-underline hover:border-ion"
                  >
                    <span className="w-12 flex-none font-mono text-xs text-text-muted">#{t.issueNumber}</span>
                    <span className="min-w-0 flex-1 truncate text-sm" title={t.title}>
                      {t.title}
                    </span>
                    {t.runStatus && <span className="text-xs text-text-muted">{runStatusLabels[t.runStatus]}</span>}
                    <span className="flex -space-x-1.5" title={t.assignees.map((a) => a.login).join(', ') || 'Unassigned'}>
                      {t.assignees.length === 0 ? (
                        <span className="text-xs text-text-muted">Unassigned</span>
                      ) : (
                        <>
                          {t.assignees.slice(0, 3).map((a) => (
                            <img key={a.login} src={a.avatarUrl} alt="" className="size-5 rounded-full border border-surface" />
                          ))}
                          {t.assignees.length > 3 && <span className="pl-2.5 font-mono text-[11px] text-text-muted">+{t.assignees.length - 3}</span>}
                        </>
                      )}
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
      {/* Pass the project on, for the drawer's read-only checks. */}
      <Outlet context={useProjectContext()} />
    </>
  )
}
