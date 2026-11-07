import { useMemo } from 'react'
import { Outlet } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { StatusMessage } from '../../components/StatusMessage'
import { useBoard } from '../../data/views/board'
import type { Board, Task } from '../../data/types'
import { columnLabels, labels } from '../../labels/labels'
import { BoardToolbar } from './BoardToolbar'
import { TaskCard } from './TaskCard'
import { TelemetryBar } from './TelemetryBar'

// Mission control, from Main.dc.html. Static cards for now; drag, filters
// and keyboard control come in feature/frontend-board. The <Outlet /> renders
// the payload drawer and the create dialogs over the board.
export function BoardPage() {
  const project = useProjectContext()
  const { data: board, isPending, error } = useBoard(project.id)

  if (isPending) return <StatusMessage title={`Loading ${labels.board.toLowerCase()}…`} />
  if (error) return <StatusMessage title="Something went wrong" body={error.message} />
  return <BoardView board={board} />
}

function BoardView({ board }: { board: Board }) {
  const lookup = useMemo(() => {
    const tasks = board.columns.flatMap((c) => c.tasks)
    const taskIds = new Set(tasks.map((t) => t.id))
    return {
      epic: new Map(board.epics.map((e) => [e.id, e])),
      feature: new Map(board.features.map((f) => [f.id, f])),
      user: new Map(board.users.map((u) => [u.id, u])),
      agent: new Map(board.agents.map((a) => [a.id, a])),
      counts: {
        awaitingApproval: tasks.filter((t) => t.column === 'user-check' && t.status !== 'landed').length,
        blocked: tasks.filter((t) => t.status === 'blocked').length,
        agentsActive: board.agents.filter((a) => a.state !== 'idle' && a.currentTaskId && taskIds.has(a.currentTaskId))
          .length,
      },
    }
  }, [board])

  const ownerOf = (t: Task) => {
    if (!t.assignee) return null
    if (t.assignee.kind === 'agent') {
      const agent = lookup.agent.get(t.assignee.id)
      return agent ? { kind: 'agent' as const, agent } : null
    }
    const user = lookup.user.get(t.assignee.id)
    return user ? { kind: 'user' as const, user } : null
  }

  return (
    <>
      <TelemetryBar project={board.project} sprint={board.sprint} counts={lookup.counts} />
      <section aria-label={labels.board} className="relative min-w-0 flex-1 overflow-x-auto px-7 py-6">
        <BoardToolbar projectId={board.project.id} features={board.features} />
        <div className="grid min-w-[1120px] grid-cols-[repeat(5,minmax(216px,1fr))] gap-4">
          {board.columns.map((col) => {
            const name = columnLabels[col.key]
            return (
              <section
                key={col.key}
                aria-label={`${name.name} · ${name.plain}, ${col.tasks.length} ${labels.tasks.toLowerCase()}`}
                className="flex min-h-[640px] flex-col gap-3 rounded-card border border-line-soft bg-surface/60 p-3"
              >
                <div className="flex items-baseline justify-between px-1 pt-1 pb-2">
                  <div>
                    <h2 className="m-0 font-heading text-[15px] font-semibold">{name.name}</h2>
                    <div className="text-xs text-text-muted">{name.plain}</div>
                  </div>
                  <span className="font-mono text-xs text-text-muted">{col.tasks.length}</span>
                </div>
                {col.tasks.map((t) => (
                  <TaskCard
                    key={t.id}
                    task={t}
                    epic={lookup.epic.get(t.epicId)}
                    feature={lookup.feature.get(t.featureId)}
                    owner={ownerOf(t)}
                    to={`/projects/${board.project.id}/tasks/${t.id}`}
                  />
                ))}
                {col.tasks.length === 0 && (
                  <p className="m-0 rounded-control border border-dashed border-line px-3 py-6 text-center text-[13px] text-text-muted">
                    No {labels.tasks.toLowerCase()} here yet
                  </p>
                )}
              </section>
            )
          })}
        </div>
      </section>
      <Outlet />
    </>
  )
}
