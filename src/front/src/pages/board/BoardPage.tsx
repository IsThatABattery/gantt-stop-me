import {
  closestCorners,
  DndContext,
  DragOverlay,
  MouseSensor,
  TouchSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { arrayMove } from '@dnd-kit/sortable'
import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { Outlet } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { StatusMessage } from '../../components/StatusMessage'
import { ConflictError } from '../../data/core'
import { useBoard, useMoveTask } from '../../data/views/board'
import { COLUMN_ORDER, type Board, type ColumnKey, type ID, type Task } from '../../data/types'
import { columnLabels, labels } from '../../labels/labels'
import { BoardColumn } from './BoardColumn'
import { BoardToolbar } from './BoardToolbar'
import { isFiltered, matches, noFilter, type BoardFilter, type CounterFilter } from './filters'
import { TaskCard } from './TaskCard'
import { TelemetryBar } from './TelemetryBar'
import { drawTrail } from './trail'

// Mission control, from Main.dc.html. Cards drag between and within columns,
// the objective select and the telemetry counters filter them, and the
// keyboard drives it all (AGENT_BRIEF §10). The <Outlet /> renders the
// payload drawer and the create dialogs over the board.
export function BoardPage() {
  const project = useProjectContext()
  const { data: board, isPending, error } = useBoard(project.id)

  if (isPending) return <StatusMessage title={`Loading ${labels.board.toLowerCase()}…`} />
  if (error) return <StatusMessage title="Something went wrong" body={error.message} />
  return <BoardView board={board} />
}

/** Card ids per column, as shown. */
type Layout = Record<ColumnKey, ID[]>

interface DragState {
  activeId: ID
  layout: Layout
  /** Where the card was when the drag started */
  origin: { column: ColumnKey; before: ID | null; rect: DOMRect | null }
}

/** A move waiting for the board to show it: then the trail draws and focus follows. */
interface PendingMove {
  taskId: ID
  column: ColumnKey
  from: DOMRect | null
  refocus: boolean
}

const MOVE_LEFT = '['
const MOVE_RIGHT = ']'

const isColumnId = (id: ID) => id.startsWith('column:')

function BoardView({ board }: { board: Board }) {
  const [filter, setFilter] = useState<BoardFilter>(noFilter)
  const [drag, setDrag] = useState<DragState | null>(null)
  const [rejection, setRejection] = useState<string | null>(null)
  const [announcement, setAnnouncement] = useState('')
  const pending = useRef<PendingMove | null>(null)
  const gridRef = useRef<HTMLDivElement>(null)

  const lookup = useMemo(() => {
    const tasks = board.columns.flatMap((c) => c.tasks)
    const taskIds = new Set(tasks.map((t) => t.id))
    return {
      task: new Map(tasks.map((t) => [t.id, t])),
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

  const visible = useMemo(
    () =>
      Object.fromEntries(
        board.columns.map((c) => [c.key, c.tasks.filter((t) => matches(t, filter)).map((t) => t.id)]),
      ) as Layout,
    [board, filter],
  )
  const layout = drag?.layout ?? visible
  const filtered = isFiltered(filter)

  const onRejected = useCallback((task: Task, error: Error) => {
    pending.current = null
    const back = columnLabels[task.column].name
    const why = error instanceof ConflictError ? 'someone else changed it first' : error.message
    setRejection(`${task.key} couldn't move: ${why}. It's back in ${back}.`)
  }, [])
  const move = useMoveTask(board.project.id, onRejected)

  const cardEl = (id: ID) => gridRef.current?.querySelector<HTMLElement>(`[data-task-id="${CSS.escape(id)}"]`) ?? null

  function requestMove(task: Task, column: ColumnKey, before: ID | null, from: DOMRect | null, refocus: boolean) {
    setRejection(null)
    pending.current = { taskId: task.id, column, from: column === task.column ? null : from, refocus }
    setAnnouncement(`${task.key} moved to ${columnLabels[column].name}.`)
    move.mutate({ taskId: task.id, column, beforeTaskId: before, version: task.version })
  }

  // Once the board shows a move: draw the trail and keep focus on the card.
  useEffect(() => {
    const p = pending.current
    if (!p || lookup.task.get(p.taskId)?.column !== p.column) return
    pending.current = null
    const el = cardEl(p.taskId)
    if (!el) return
    if (p.from) drawTrail(p.from, el.getBoundingClientRect())
    if (p.refocus) el.focus()
  }, [lookup])

  // ---- Drag and drop ----

  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 6 } }),
  )

  const containerOf = (id: ID, l: Layout): ColumnKey | undefined =>
    isColumnId(id) ? (id.slice('column:'.length) as ColumnKey) : COLUMN_ORDER.find((k) => l[k].includes(id))

  function onDragStart({ active }: DragStartEvent) {
    const id = String(active.id)
    const column = containerOf(id, visible)
    if (!column) return
    const ids = visible[column]
    setRejection(null)
    setDrag({
      activeId: id,
      layout: visible,
      origin: { column, before: ids[ids.indexOf(id) + 1] ?? null, rect: cardEl(id)?.getBoundingClientRect() ?? null },
    })
  }

  // Crossing into another column moves the card's placeholder there.
  function onDragOver({ active, over }: DragOverEvent) {
    if (!over) return
    setDrag((d) => {
      if (!d) return d
      const id = String(active.id)
      const overId = String(over.id)
      const from = containerOf(id, d.layout)
      const to = containerOf(overId, d.layout)
      if (!from || !to || from === to) return d
      const target = d.layout[to].filter((x) => x !== id)
      const overIndex = target.indexOf(overId)
      const translated = active.rect.current.translated
      const below = translated && translated.top > over.rect.top + over.rect.height / 2
      const at = overIndex === -1 ? target.length : overIndex + (below ? 1 : 0)
      target.splice(at, 0, id)
      return { ...d, layout: { ...d.layout, [from]: d.layout[from].filter((x) => x !== id), [to]: target } }
    })
  }

  function onDragEnd({ active, over }: DragEndEvent) {
    const d = drag
    setDrag(null)
    if (!d || !over) return
    const id = String(active.id)
    const column = containerOf(id, d.layout)
    const task = lookup.task.get(id)
    if (!column || !task) return
    let ids = d.layout[column]
    const overId = String(over.id)
    if (!isColumnId(overId) && overId !== id && ids.includes(overId)) {
      ids = arrayMove(ids, ids.indexOf(id), ids.indexOf(overId))
    }
    const before = ids[ids.indexOf(id) + 1] ?? null
    if (column === d.origin.column && before === d.origin.before) return
    requestMove(task, column, before, d.origin.rect, false)
  }

  // ---- Keyboard ----

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (drag || e.altKey || e.ctrlKey || e.metaKey) return
    const id = (e.target as HTMLElement).closest('[data-task-id]')?.getAttribute('data-task-id')
    if (!id) return
    const column = containerOf(id, visible)
    const task = lookup.task.get(id)
    if (!column || !task) return
    const row = visible[column].indexOf(id)
    const col = COLUMN_ORDER.indexOf(column)

    const focusCard = (target: ID | undefined) => {
      if (!target) return
      e.preventDefault()
      cardEl(target)?.focus()
    }

    switch (e.key) {
      case 'ArrowUp':
        return focusCard(visible[column][row - 1])
      case 'ArrowDown':
        return focusCard(visible[column][row + 1])
      case 'ArrowLeft':
      case 'ArrowRight': {
        const step = e.key === 'ArrowLeft' ? -1 : 1
        for (let c = col + step; c >= 0 && c < COLUMN_ORDER.length; c += step) {
          const ids = visible[COLUMN_ORDER[c]]
          if (ids.length) return focusCard(ids[Math.min(row, ids.length - 1)])
        }
        return
      }
      case MOVE_LEFT:
      case MOVE_RIGHT: {
        const target = COLUMN_ORDER[col + (e.key === MOVE_LEFT ? -1 : 1)]
        if (!target) return
        e.preventDefault()
        // Lands at the same height in the next column, or at the bottom.
        requestMove(task, target, visible[target][row] ?? null, cardEl(id)?.getBoundingClientRect() ?? null, true)
        return
      }
    }
  }

  // ---- Render ----

  const ownerOf = (t: Task) => {
    if (!t.assignee) return null
    if (t.assignee.kind === 'agent') {
      const agent = lookup.agent.get(t.assignee.id)
      return agent ? { kind: 'agent' as const, agent } : null
    }
    const user = lookup.user.get(t.assignee.id)
    return user ? { kind: 'user' as const, user } : null
  }

  const card = (t: Task, overlay = false) => (
    <TaskCard
      task={t}
      epic={lookup.epic.get(t.epicId)}
      feature={lookup.feature.get(t.featureId)}
      owner={ownerOf(t)}
      to={`/projects/${board.project.id}/tasks/${t.id}`}
      overlay={overlay}
    />
  )

  const toggleCounter = (counter: CounterFilter) =>
    setFilter((f) => ({ ...f, counter: f.counter === counter ? null : counter }))

  const dragged = drag ? lookup.task.get(drag.activeId) : undefined

  return (
    <>
      <TelemetryBar
        project={board.project}
        sprint={board.sprint}
        counts={lookup.counts}
        active={filter.counter}
        onToggle={toggleCounter}
      />
      <section aria-label={labels.board} className="relative min-w-0 flex-1 overflow-x-auto px-7 py-6">
        <BoardToolbar
          projectId={board.project.id}
          features={board.features}
          featureId={filter.featureId}
          onFeatureChange={(featureId) => setFilter((f) => ({ ...f, featureId }))}
          onClear={filtered ? () => setFilter(noFilter) : undefined}
        />
        {rejection && (
          <div
            role="alert"
            className="mb-4 flex items-center justify-between gap-3 rounded-control border border-critical/60 bg-critical/8 px-3.5 py-2 text-[13px] text-critical"
          >
            <span>{rejection}</span>
            <button
              type="button"
              onClick={() => setRejection(null)}
              className="min-h-11 cursor-pointer rounded-control px-3 text-critical hover:underline"
            >
              Dismiss
            </button>
          </div>
        )}
        <p aria-live="polite" className="sr-only">
          {announcement}
        </p>
        <DndContext
          sensors={sensors}
          collisionDetection={closestCorners}
          onDragStart={onDragStart}
          onDragOver={onDragOver}
          onDragEnd={onDragEnd}
          onDragCancel={() => setDrag(null)}
        >
          <div
            ref={gridRef}
            onKeyDown={onKeyDown}
            className="grid min-w-[1120px] grid-cols-[repeat(5,minmax(216px,1fr))] gap-4"
          >
            {COLUMN_ORDER.map((key) => (
              <BoardColumn
                key={key}
                columnKey={key}
                tasks={layout[key].flatMap((id) => lookup.task.get(id) ?? [])}
                total={board.columns.find((c) => c.key === key)?.tasks.length ?? 0}
                filtered={filtered}
                renderCard={(t) => card(t)}
              />
            ))}
          </div>
          <DragOverlay>{dragged ? card(dragged, true) : null}</DragOverlay>
        </DndContext>
      </section>
      <Outlet />
    </>
  )
}
