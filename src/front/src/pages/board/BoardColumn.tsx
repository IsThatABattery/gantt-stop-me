import { useDroppable } from '@dnd-kit/core'
import { SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { ReactNode } from 'react'
import { Link, useParams } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Icon } from '../../components/Icon'
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
  const { projectId } = useParams()

  return (
    <section
      ref={setNodeRef}
      aria-label={`${name.name} · ${name.plain}, ${count} ${labels.tasks.toLowerCase()}`}
      className={`flex min-h-[640px] flex-col gap-3 rounded-card border bg-surface/60 p-3 transition-colors ${
        isOver ? 'border-ion/60' : 'border-line-soft'
      }`}
    >
      {columnKey === 'backlog' && (
        // Planning is how work enters Backlog, so its way in sits on top of it.
        <Link to={`/projects/${projectId}/plans`} className={buttonClass('primary', 'w-full justify-center gap-2')}>
          <Icon name="target" size={16} />
          {labels.plans}
        </Link>
      )}
      <div className="flex items-baseline justify-between px-1 pt-1 pb-2">
        <div>
          <h2 className="m-0 font-heading text-[15px] font-semibold">{name.name}</h2>
          <div className="text-xs text-text-muted">{name.plain}</div>
        </div>
        <span className="font-mono text-xs text-text-muted" aria-hidden="true">
          {count}
        </span>
      </div>
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
