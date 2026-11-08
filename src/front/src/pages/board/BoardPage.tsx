import {
  closestCorners,
  pointerWithin,
  type CollisionDetection,
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
import { Button } from '../../components/controls'
import type { Task } from '../../data/api/types'
import { ConflictError } from '../../data/errors'
import { COLUMN_ORDER, type ColumnKey } from '../../data/types'
import { useBoard, useMoveTask, type Board, type BoardColumn as Column } from '../../data/views/board'
import { columnLabels, labels } from '../../labels/labels'
import { BoardColumn } from './BoardColumn'
import { BoardToolbar } from './BoardToolbar'
import { countAll, isFiltered, matches, noFilter, type BoardFilter, type CounterFilter } from './filters'
import { TaskCard } from './TaskCard'
import { TelemetryBar } from './TelemetryBar'
import { drawTrail } from './trail'

// Mission control, from Main.dc.html. Cards are the repository's issues. They
// drag between and within columns, the telemetry counters filter them, and
// the keyboard drives it all (AGENT_BRIEF §10). The <Outlet /> renders the
// payload drawer and the create dialogs over the board.
export function BoardPage() {
  const project = useProjectContext()
  const { data: board, isPending, error, refetch } = useBoard(String(project.id))

  if (isPending) return <StatusMessage title={`Loading ${labels.board.toLowerCase()}…`} />
  if (error && !board)
    return (
      <StatusMessage title="Something went wrong" body={error.message}>
        <Button onClick={() => refetch()}>Try again</Button>
      </StatusMessage>
    )
  return <BoardView board={board} />
}

/** Card ids per column, as shown. Ids are strings here, as drag and drop needs. */
type Layout = Record<ColumnKey, string[]>

interface DragState {
  activeId: string
  layout: Layout
  /** Where the card was when the drag started */
  origin: { column: ColumnKey; before: string | null; rect: DOMRect | null }
}

/** A move waiting for the board to show it: then the trail draws and focus follows. */
interface PendingMove {
  taskId: string
  column: ColumnKey
  from: DOMRect | null
  refocus: boolean
}

const MOVE_LEFT = '['
const MOVE_RIGHT = ']'

const isColumnId = (id: string) => id.startsWith('column:')

function BoardView({ board }: { board: Board }) {
  const [filter, setFilter] = useState<BoardFilter>(noFilter)
  const [drag, setDrag] = useState<DragState | null>(null)
  const [rejection, setRejection] = useState<string | null>(null)
  const [announcement, setAnnouncement] = useState('')
  const pending = useRef<PendingMove | null>(null)
  const gridRef = useRef<HTMLDivElement>(null)

  const lookup = useMemo(() => {
    const tasks = board.columns.flatMap((c) => c.tasks)
    return {
      task: new Map(tasks.map((t) => [String(t.id), t])),
      column: new Map<ColumnKey, Column>(board.columns.map((c) => [c.key, c])),
      keyOf: new Map(board.columns.map((c) => [c.id, c.key])),
      counts: countAll(board.columns),
    }
  }, [board])
  const columnOf = (t: Task) => lookup.keyOf.get(t.columnId)

  const visible = useMemo(() => {
    const ids = (key: ColumnKey) =>
      (board.columns.find((c) => c.key === key)?.tasks ?? []).filter((t) => matches(t, key, filter)).map((t) => String(t.id))
    return Object.fromEntries(COLUMN_ORDER.map((key) => [key, ids(key)])) as Layout
  }, [board, filter])
  const layout = drag?.layout ?? visible
  const filtered = isFiltered(filter)

  const onRejected = useCallback(
    (task: Task, error: Error) => {
      pending.current = null
      const back = lookup.keyOf.get(task.columnId)
      const why = error instanceof ConflictError && error.code === 'conflict' ? 'someone else changed it first' : error.message
      const sentence = /[.!?]$/.test(why) ? why : `${why}.`
      setRejection(`#${task.issueNumber} couldn't move: ${sentence}${back ? ` It's back in ${columnLabels[back].name}.` : ''}`)
    },
    [lookup],
  )
  const move = useMoveTask(String(board.project.id), onRejected)

  const cardEl = (id: string) => gridRef.current?.querySelector<HTMLElement>(`[data-task-id="${CSS.escape(id)}"]`) ?? null

  function requestMove(task: Task, column: ColumnKey, before: string | null, from: DOMRect | null, refocus: boolean) {
    const target = lookup.column.get(column)
    if (!target) return
    setRejection(null)
    pending.current = { taskId: String(task.id), column, from: column === columnOf(task) ? null : from, refocus }
    setAnnouncement(`#${task.issueNumber} moved to ${columnLabels[column].name}.`)
    move.mutate({ taskId: task.id, columnId: target.id, beforeTaskId: before === null ? null : Number(before), version: task.version })
  }

  // Once the board shows a move: draw the trail and keep focus on the card.
  useEffect(() => {
    const p = pending.current
    const task = p && lookup.task.get(p.taskId)
    if (!p || !task || lookup.keyOf.get(task.columnId) !== p.column) return
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

  // What's under the pointer wins: closestCorners alone lets nearby cards beat an empty column, whose corners are
  // far away, so a card could never be dropped into one.
  const collision: CollisionDetection = (args) => {
    const hits = pointerWithin(args)
    return hits.length ? hits : closestCorners(args)
  }

  const containerOf = (id: string, l: Layout): ColumnKey | undefined =>
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

    const focusCard = (target: string | undefined) => {
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

  const card = (t: Task, overlay = false) => <TaskCard task={t} to={`/projects/${board.project.id}/tasks/${t.id}`} overlay={overlay} />

  const toggleCounter = (counter: CounterFilter) =>
    setFilter((f) => ({ ...f, counter: f.counter === counter ? null : counter }))

  const dragged = drag ? lookup.task.get(drag.activeId) : undefined

  return (
    <>
      <TelemetryBar
        project={board.project}
        counts={lookup.counts}
        active={filter.counter}
        onToggle={toggleCounter}
      />
      <section aria-label={labels.board} className="relative min-w-0 flex-1 overflow-x-auto px-7 py-6">
        <BoardToolbar projectId={board.project.id} onClear={filtered ? () => setFilter(noFilter) : undefined} />
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
          collisionDetection={collision}
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
                total={lookup.column.get(key)?.tasks.length ?? 0}
                filtered={filtered}
                renderCard={(t) => card(t)}
              />
            ))}
          </div>
          <DragOverlay>{dragged ? card(dragged, true) : null}</DragOverlay>
        </DndContext>
      </section>
      {/* The drawer and dialogs read the project too. */}
      <Outlet context={board.project} />
    </>
  )
}
