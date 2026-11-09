import { useDroppable } from '@dnd-kit/core'
import { SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { CSSProperties, ReactNode } from 'react'
import { Icon } from '../../components/Icon'
import { InkTab } from '../../components/Tile'
import type { Task } from '../../data/api/types'
import type { ColumnKey } from '../../data/types'
import { columnPurposeLabels } from '../../labels/labels'
import { APPROVAL_COLUMN, needsPerson, taskCount } from './filters'

/** Droppable id of a column, apart from card ids. */
const columnDropId = (key: ColumnKey) => `column:${key}`

// A column the label dictionary doesn't know yet gets no purpose line, and its row still lines up with the others.
const purposes = columnPurposeLabels

interface BoardColumnProps {
  columnKey: ColumnKey
  name: string
  /** The tile's shade, 0 for the first column and 1 for the last: tileStep(index, count) */
  step: number
  /** The tasks shown, in order */
  tasks: Task[]
  /** Their ids, as drag and drop knows them. Pass the layout's own array: a new one each render makes the sortable
   *  context re-measure on every render, which loops while a card is dragged. */
  ids: string[]
  /** Every task in the column, before filters: the tab's mark counts these */
  all: Task[]
  filtered: boolean
  /** The active filter's name, for the empty state */
  filterLabel?: string
  /** A dragged card would land here */
  dropTarget: boolean
  /** Read-only members can't move cards, so the empty column doesn't offer it */
  canWrite: boolean
  renderCard: (task: Task) => ReactNode
}

/**
 * One column as a stage tile: a stepped neutral fill, its solid ink tab, then inside the keyline frame its purpose line
 * and the cards. Columns are subgrid items of the board, so tabs, purpose lines and first cards line up across them.
 */
export function BoardColumn({ columnKey, name, step, tasks, ids, all, filtered, filterLabel, dropTarget, canWrite, renderCard }: BoardColumnProps) {
  const { setNodeRef } = useDroppable({ id: columnDropId(columnKey) })

  const needing = all.filter(needsPerson).length
  const awaiting = columnKey === APPROVAL_COLUMN ? all.length : 0
  const mark = needing ? 'signal' : awaiting ? 'approve' : undefined
  const markLabel = needing ? `${needing} ${needing === 1 ? 'needs' : 'need'} attention` : awaiting ? `${awaiting} awaiting approval` : undefined
  const count = filtered ? `${tasks.length} of ${all.length}` : String(tasks.length)
  const purpose = purposes[columnKey]
  // An empty column says why it's empty, or how to fill it, if this member can.
  const hint = filtered && filterLabel ? `Filtered to ${filterLabel}` : canWrite ? 'Drag a task here, or move one with [ and ].' : undefined

  return (
    <section
      ref={setNodeRef}
      aria-label={`${name}, ${filtered ? taskCount(tasks.length, all.length) : taskCount(tasks.length)}${markLabel ? `, ${markLabel}` : ''}`}
      className="tile row-span-3 grid min-w-0 grid-rows-subgrid gap-y-1.5 rounded-card p-1.5"
      style={{ '--tile-step': step } as CSSProperties}
    >
      <InkTab count={count} countLabel={filtered ? 'tasks' : tasks.length === 1 ? 'task' : 'tasks'} mark={mark} markLabel={markLabel}>
        {name}
      </InkTab>

      <div
        className={`row-span-2 grid min-w-0 grid-rows-subgrid gap-y-2.5 rounded-[3px] border p-2 transition-colors duration-150 ease-out-quint ${
          dropTarget ? 'border-ink' : 'border-tile-keyline'
        }`}
      >
        {purpose &&
          (mark === 'approve' ? (
            // Work waits on a person here: the purpose becomes the yellow approve band.
            <p className="m-0 flex items-center gap-1.5 rounded-[2px] bg-approve px-2 py-[3px] text-[13px] leading-[1.3] font-semibold text-balance text-on-approve">
              <Icon name="person" size={14} className="flex-none" />
              {purpose}
            </p>
          ) : (
            <p className="m-0 flex min-h-[26px] items-center text-[13px] leading-[1.3] text-balance text-ink-2">{purpose}</p>
          ))}

        <SortableContext id={columnDropId(columnKey)} items={ids} strategy={verticalListSortingStrategy}>
          {tasks.length > 0 ? (
            // The board is at most a screen tall, so a long column scrolls its own cards. The 4px band around them
            // (padding, cancelled by the margin) keeps a focused card's outline inside the scroll box's clip.
            <ul className="row-start-2 -m-1 flex min-h-0 min-w-0 list-none flex-col gap-2 overflow-y-auto p-1">
              {tasks.map((t) => (
                <SortableCard key={t.id} id={String(t.id)}>
                  {renderCard(t)}
                </SortableCard>
              ))}
            </ul>
          ) : (
            <div className="row-start-2 flex flex-col items-center gap-1 self-start rounded-[3px] border border-ink/25 px-3 py-5 text-center">
              <p className="m-0 text-[13px] font-bold text-ink">{filtered ? 'No matches' : 'No tasks'}</p>
              {hint && <p className="m-0 text-[12.5px] leading-[1.4] text-ink-2">{hint}</p>}
            </div>
          )}
        </SortableContext>
      </div>
    </section>
  )
}

// Only the pointer listeners are spread: the card's own link stays the one focusable element, and the board handles
// the keyboard itself. While its card is dragged, the item stays behind as the slot where it will land.
function SortableCard({ id, children }: { id: string; children: ReactNode }) {
  const { setNodeRef, listeners, transform, transition, isDragging } = useSortable({ id })
  return (
    <li
      ref={setNodeRef}
      {...listeners}
      className={`min-w-0 touch-manipulation rounded-card ${
        isDragging ? 'bg-ink/8 outline-[1.5px] -outline-offset-[1.5px] outline-ink outline-dashed [&>*]:invisible' : ''
      }`}
      style={{ transform: CSS.Translate.toString(transform), transition }}
    >
      {children}
    </li>
  )
}
