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
  type UniqueIdentifier,
} from '@dnd-kit/core'
import { arrayMove } from '@dnd-kit/sortable'
import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react'
import { Outlet } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { Button } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { tileStep } from '../../components/tileStep'
import type { Task } from '../../data/api/types'
import { ConflictError } from '../../data/errors'
import type { ColumnKey } from '../../data/types'
import { useBoard, useMoveTask, type Board, type BoardColumn as Column } from '../../data/views/board'
import { columnLabels, labels } from '../../labels/labels'
import { BoardColumn } from './BoardColumn'
import { BoardHeader } from './BoardHeader'
import { APPROVAL_COLUMN, counters, countAll, isFiltered, matches, noFilter, type BoardFilter, type CounterFilter } from './filters'
import { TaskCard } from './TaskCard'
import { drawTrail } from './trail'

// The board as a process-flow sheet: the project's columns as stepped tiles under ink tabs, read left to right, with
// the repository's issues as white keylined cards. Cards drag between and within columns, the header's counters
// filter them, and the keyboard drives it all. The columns come from the API in its order, so nothing here depends on
// there being five. The <Outlet /> renders the task drawer and the create dialogs over the board.
export function BoardPage() {
  const project = useProjectContext()
  const { data: board, isPending, error, refetch, isRefetching } = useBoard(String(project.id))

  if (isPending) return <BoardSkeleton />
  // Only a failed first load ends here: a failed refetch keeps the board that already loaded.
  if (error && !board)
    return (
      <StatusMessage tone="signal" title="The board didn’t load" body={error.message}>
        <Button variant="primary" busy={isRefetching} onClick={() => void refetch()}>
          {isRefetching ? 'Trying again…' : 'Try again'}
        </Button>
      </StatusMessage>
    )
  return <BoardView board={board} />
}

/** A column's name from the label set, or the API's name for a column the labels don't know yet */
const nameOf = (column: Pick<Column, 'key' | 'name'>) => (columnLabels as Partial<Record<string, { name: string }>>)[column.key]?.name ?? column.name

/** The page's frame and gutters. These three are shared by the board and its skeleton, so loading never jumps. */
const PAGE = 'flex min-w-0 flex-1 flex-col px-4 pt-5 pb-4 md:px-7'
/** Bleeds to the page edge so a column cut off by the viewport runs to it, then scrolls sideways; the page never does */
const SCROLLER = 'relative -mx-4 mt-5 flex flex-1 overflow-x-auto px-4 pb-1 md:-mx-7 md:px-7'
/**
 * Any number of columns, each at least 220px (272px on phones) and growing to fill; rows: tab, purpose, cards. At most
 * a screen tall: a long column scrolls its own cards, so the sideways scrollbar under the tiles stays in reach.
 */
const TRACKS =
  'grid max-h-[calc(100dvh-2rem)] flex-1 grid-flow-col auto-cols-[minmax(17rem,1fr)] grid-rows-[auto_auto_minmax(0,1fr)] gap-x-3 md:auto-cols-[minmax(13.75rem,1fr)]'

/** Card ids per column key, as shown. Ids are strings here, as drag and drop needs. */
type Layout = Record<string, string[]>

const NONE: string[] = []

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
    const column = new Map<ColumnKey, Column>(board.columns.map((c) => [c.key, c]))
    return {
      /** Column keys in board order */
      order: board.columns.map((c) => c.key),
      task: new Map(tasks.map((t) => [String(t.id), t])),
      column,
      name: (key: ColumnKey) => {
        const c = column.get(key)
        return c ? nameOf(c) : key
      },
      keyOf: new Map(board.columns.map((c) => [c.id, c.key])),
      counts: countAll(board.columns),
      total: tasks.length,
    }
  }, [board])
  const { order } = lookup
  const columnOf = (t: Task) => lookup.keyOf.get(t.columnId)

  const visible = useMemo(
    () => Object.fromEntries(board.columns.map((c) => [c.key, c.tasks.filter((t) => matches(t, c.key, filter)).map((t) => String(t.id))])) as Layout,
    [board, filter],
  )
  const layout = drag?.layout ?? visible
  const filtered = isFiltered(filter)
  const canWrite = board.project.permission !== 'read'

  // `task` is where the card lands: where it was, or for a conflict where the API says it is.
  const onRejected = useCallback(
    (task: Task, error: Error) => {
      const back = lookup.keyOf.get(task.columnId)
      // The rollback remounts the card, so focus follows it back if it was on it.
      const focused = document.activeElement?.getAttribute('data-task-id') === String(task.id)
      pending.current = focused && back ? { taskId: String(task.id), column: back, from: null, refocus: true } : null
      const conflict = error instanceof ConflictError && error.code === 'conflict'
      const why = conflict ? 'someone else changed it first' : error.message
      const sentence = /[.!?]$/.test(why) ? why : `${why}.`
      setRejection(`#${task.issueNumber} couldn't move: ${sentence}${back ? ` It's ${conflict ? '' : 'back '}in ${lookup.name(back)}.` : ''}`)
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
    setAnnouncement(`#${task.issueNumber} moved to ${lookup.name(column)}.`)
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
  // far away, so a card could never be dropped into one. Between columns, or off the board, the last target holds:
  // guessing the nearest corners there flips between two neighbours as the card's slot moves, and that loops.
  const lastOver = useRef<UniqueIdentifier | null>(null)
  const collision: CollisionDetection = (args) => {
    const hits = pointerWithin(args)
    if (hits.length) {
      lastOver.current = hits[0].id
      return hits
    }
    if (lastOver.current !== null && args.droppableContainers.some((c) => c.id === lastOver.current)) return [{ id: lastOver.current }]
    return closestCorners(args)
  }

  const containerOf = (id: string, l: Layout): ColumnKey | undefined =>
    isColumnId(id) ? (id.slice('column:'.length) as ColumnKey) : order.find((k) => l[k]?.includes(id))

  function onDragStart({ active }: DragStartEvent) {
    const id = String(active.id)
    const column = containerOf(id, visible)
    if (!column) return
    const ids = visible[column]
    lastOver.current = null
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
    const col = order.indexOf(column)

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
        for (let c = col + step; c >= 0 && c < order.length; c += step) {
          const ids = visible[order[c]]
          if (ids.length) return focusCard(ids[Math.min(row, ids.length - 1)])
        }
        return
      }
      case MOVE_LEFT:
      case MOVE_RIGHT: {
        const target = order[col + (e.key === MOVE_LEFT ? -1 : 1)]
        if (!target || !canWrite) return
        e.preventDefault()
        // Lands at the same height in the next column, or at the bottom.
        requestMove(task, target, visible[target][row] ?? null, cardEl(id)?.getBoundingClientRect() ?? null, true)
        return
      }
    }
  }

  // ---- Render ----

  const card = (t: Task, overlay = false) => (
    <TaskCard
      task={t}
      to={`/projects/${board.project.id}/tasks/${t.id}`}
      awaitingApproval={columnOf(t) === APPROVAL_COLUMN}
      overlay={overlay}
    />
  )

  const toggleCounter = (counter: CounterFilter) => setFilter((f) => ({ ...f, counter: f.counter === counter ? null : counter }))

  const dragged = drag ? lookup.task.get(drag.activeId) : undefined
  const dropColumn = drag ? containerOf(drag.activeId, drag.layout) : undefined
  const shown = order.reduce((n, key) => n + (visible[key]?.length ?? 0), 0)
  const filterLabel = counters.find((c) => c.filter === filter.counter)?.label

  return (
    <>
      <section aria-label={labels.board} className={PAGE}>
        <BoardHeader
          project={board.project}
          counts={lookup.counts}
          total={lookup.total}
          shown={shown}
          active={filter.counter}
          canWrite={canWrite}
          onToggle={toggleCounter}
          onClear={() => setFilter(noFilter)}
        />
        <p aria-live="polite" className="sr-only">
          {announcement}
        </p>
        <DndContext
          sensors={canWrite ? sensors : []}
          collisionDetection={collision}
          onDragStart={onDragStart}
          onDragOver={onDragOver}
          onDragEnd={onDragEnd}
          onDragCancel={() => setDrag(null)}
        >
          <div className={SCROLLER}>
            <div ref={gridRef} onKeyDown={onKeyDown} className={TRACKS}>
              {board.columns.map((column, i) => (
                <BoardColumn
                  key={column.key}
                  columnKey={column.key}
                  name={nameOf(column)}
                  step={tileStep(i, board.columns.length)}
                  tasks={(layout[column.key] ?? NONE).flatMap((id) => lookup.task.get(id) ?? [])}
                  ids={layout[column.key] ?? NONE}
                  all={column.tasks}
                  filtered={filtered}
                  filterLabel={filterLabel}
                  dropTarget={dropColumn === column.key}
                  canWrite={canWrite}
                  renderCard={(t) => card(t)}
                />
              ))}
            </div>
          </div>
          <DragOverlay>{dragged ? card(dragged, true) : null}</DragOverlay>
        </DndContext>
      </section>
      {/* Out of flow, so it never shifts the columns under the pointer and shows at any scroll position. It floats
          above the page and closes, so it takes the menu shadow. */}
      {rejection && (
        <div
          role="alert"
          className="fixed right-4 bottom-4 left-4 z-40 flex items-center gap-3 rounded-card border border-signal bg-signal-wash py-1 pr-1 pl-2 text-[13px] text-signal-ink shadow-lg sm:left-auto sm:max-w-md"
        >
          <span aria-hidden="true" className="grid size-7 flex-none place-items-center rounded-[2px] bg-signal text-on-signal">
            <Icon name="alert" size={16} />
          </span>
          <span className="min-w-0 flex-1 font-medium wrap-anywhere">{rejection}</span>
          <button
            type="button"
            onClick={() => setRejection(null)}
            className="min-h-11 flex-none cursor-pointer rounded-control border-0 bg-transparent px-3 font-[inherit] text-[13px] font-semibold text-signal-ink underline decoration-1 underline-offset-[3px] hover:bg-signal/10 hover:decoration-2 active:bg-signal/20"
          >
            Dismiss
          </button>
        </div>
      )}
      {/* The drawer and dialogs read the project too. */}
      <Outlet context={board.project} />
    </>
  )
}

/**
 * The board's frame with nothing in it yet: a title block, blank tabs over the stepped tiles, and blank cards. The project
 * and the board share one query, so ProjectLayout can show this while it loads on the board route.
 */
export function BoardSkeleton() {
  const columns = 5
  return (
    <div className={PAGE} aria-busy="true">
      <p role="status" className="sr-only">
        Loading {labels.board.toLowerCase()}…
      </p>
      <div aria-hidden="true" className="h-[38px] w-48 rounded-[3px] bg-block md:h-[50px] md:w-64" />
      <div aria-hidden="true" className="mt-3.5 h-[18px] w-52 rounded-[2px] bg-block" />
      <div aria-hidden="true" className={`${SCROLLER} overflow-x-hidden`}>
        <div className={TRACKS}>
          {Array.from({ length: columns }, (_, i) => (
            <div
              key={i}
              className="tile row-span-3 grid grid-rows-subgrid gap-y-1.5 rounded-card p-1.5"
              style={{ '--tile-step': tileStep(i, columns) } as CSSProperties}
            >
              <div className="h-10 rounded-[3px] bg-ink/15" />
              <div className="row-span-2 grid grid-rows-subgrid gap-y-2.5 rounded-[3px] border border-tile-keyline p-2">
                <div className="h-[26px] w-3/5 rounded-[2px] bg-ink/8" />
                {/* Clipped, like a real column's cards, when the screen is shorter than the stack */}
                <div className="flex min-h-0 flex-col gap-2 overflow-hidden">
                  {Array.from({ length: 4 - (i % 3) }, (_, j) => (
                    <div key={j} className="h-24 flex-none rounded-card border border-tile-keyline bg-paper/55" />
                  ))}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
