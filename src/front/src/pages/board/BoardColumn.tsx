import { useDroppable } from '@dnd-kit/core'
import { SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { ReactNode } from 'react'
import type { Task } from '../../data/api/types'
import type { ColumnKey } from '../../data/types'
import { columnLabels, labels } from '../../labels/labels'

/** Droppable id of a column, apart from card ids. */
const columnDropId = (key: ColumnKey) => `column:${key}`

interface BoardColumnProps {
  columnKey: ColumnKey
  tasks: Task[]
  /** Cards in the column before filters */
  total: number
  filtered: boolean
  renderCard: (task: Task) => ReactNode
}

export function BoardColumn({ columnKey, tasks, total, filtered, renderCard }: BoardColumnProps) {
  const name = columnLabels[columnKey]
  const { setNodeRef, isOver } = useDroppable({ id: columnDropId(columnKey) })
  const count = filtered ? `${tasks.length} of ${total}` : `${tasks.length}`

  return (
    <section
      ref={setNodeRef}
      aria-label={`${name.name} · ${name.plain}, ${count} ${labels.tasks.toLowerCase()}`}
      className={`flex max-h-[calc(100dvh-2rem)] min-h-[640px] flex-col gap-3 rounded-card border bg-surface/60 p-3 transition-colors ${
        isOver ? 'border-ion/60' : 'border-line-soft'
      }`}
    >
      <div className="flex items-baseline justify-between px-1 pt-1 pb-2">
        <div>
          <h2 className="m-0 font-heading text-[15px] font-semibold">{name.name}</h2>
          <div className="text-xs text-text-muted">{name.plain}</div>
        </div>
        <span className="font-mono text-xs text-text-muted" aria-hidden="true">
          {count}
        </span>
      </div>
      {/* At most a viewport tall: the cards scroll under the header, and the board's sideways scrollbar stays in reach.
          pt-px leaves room for a card's hover lift. */}
      <div className="-mt-px flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto pt-px">
        <SortableContext id={columnDropId(columnKey)} items={tasks.map((t) => String(t.id))} strategy={verticalListSortingStrategy}>
          {tasks.map((t) => (
            <SortableCard key={t.id} id={String(t.id)}>
              {renderCard(t)}
            </SortableCard>
          ))}
        </SortableContext>
        {tasks.length === 0 && (
          <p className="m-0 rounded-control border border-dashed border-line px-3 py-6 text-center text-[13px] text-text-muted">
            {filtered ? `No matching ${labels.tasks.toLowerCase()}` : `No ${labels.tasks.toLowerCase()} here yet`}
          </p>
        )}
      </div>
    </section>
  )
}

// Only the pointer listeners are spread: the card's own link stays the one
// focusable element, and the board handles the keyboard itself.
function SortableCard({ id, children }: { id: string; children: ReactNode }) {
  const { setNodeRef, listeners, transform, transition, isDragging } = useSortable({ id })
  return (
    <div
      ref={setNodeRef}
      {...listeners}
      className={`touch-manipulation ${isDragging ? 'opacity-40' : ''}`}
      style={{ transform: CSS.Translate.toString(transform), transition }}
    >
      {children}
    </div>
  )
}
