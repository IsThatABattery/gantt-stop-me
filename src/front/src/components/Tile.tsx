import type { CSSProperties, ElementType, HTMLAttributes, ReactNode } from 'react'
import { cx } from '../lib/cx'
import { Icon } from './Icon'

interface TileProps extends Omit<HTMLAttributes<HTMLElement>, 'children'> {
  /** 0 (tile-first) to 1 (tile-last): tileStep(index, count) */
  step?: number
  /** The ink tab on top, usually an InkTab. The content goes below it, inside the keyline frame. */
  tab?: ReactNode
  /** The inset light keyline around the content, as in the process-flow reference; on by default */
  frame?: boolean
  as?: ElementType
  /** Classes for the frame (or the content area without one) */
  frameClassName?: string
  children?: ReactNode
}

/**
 * A stage or panel tile: a stepped neutral fill, 4px radius, 6px padding, an ink tab on top, and the content inside
 * an inset keyline frame.
 *
 *   <Tile step={tileStep(i, n)} tab={<InkTab count={4}>Testing</InkTab>}>…cards…</Tile>
 */
export function Tile({ step = 0, tab, frame = true, as: As = 'div', className = '', frameClassName = '', style, children, ...rest }: TileProps) {
  return (
    <As className={cx('tile flex min-w-0 flex-col gap-1.5 rounded-card p-1.5', className)} style={{ '--tile-step': step, ...style } as CSSProperties} {...rest}>
      {tab}
      <div className={cx(`flex min-h-0 flex-1 flex-col rounded-[3px] ${frame ? 'border border-tile-keyline p-2' : ''}`, frameClassName)}>{children}</div>
    </As>
  )
}

interface InkTabProps {
  children: ReactNode
  /** The count box on the right, e.g. 4 or "2 of 4". Give screen readers its unit with countLabel. */
  count?: ReactNode
  /** Read after the count, e.g. "tasks" */
  countLabel?: string
  /** Lights the tab when its stage holds work for a person: a signal or approve square with its icon */
  mark?: 'signal' | 'approve'
  /** Read for the mark, e.g. "1 needs attention" */
  markLabel?: string
  /** The heading level; h2 by default */
  as?: 'h1' | 'h2' | 'h3'
  id?: string
  className?: string
}

/**
 * The solid ink label tab that tops a stage or panel tile: display caps on ink, a count box, and an optional mark.
 * 40px tall, 3px radius. This is the one place besides page titles where the display face is used.
 */
export function InkTab({ children, count, countLabel, mark, markLabel, as: Heading = 'h2', id, className = '' }: InkTabProps) {
  return (
    <div className={cx('flex h-10 flex-none items-center justify-between gap-2 rounded-[3px] bg-ink pr-2.5 pl-3 text-on-ink', className)}>
      <Heading id={id} className="m-0 min-w-0 truncate font-display text-[21px] leading-none font-extrabold tracking-[0.045em] uppercase">
        {children}
      </Heading>
      <span className="flex flex-none items-center gap-1.5">
        {mark && (
          <span
            className={`grid size-6 place-items-center rounded-[2px] ${mark === 'signal' ? 'bg-signal text-on-signal' : 'bg-approve text-on-approve'}`}
            role={markLabel ? 'img' : undefined}
            aria-label={markLabel}
            aria-hidden={markLabel ? undefined : true}
          >
            <Icon name={mark === 'signal' ? 'alert' : 'person'} size={14} />
          </span>
        )}
        {count !== undefined && (
          <span className="min-w-6 rounded-[2px] border border-on-ink/50 px-1.5 text-center text-[13px] leading-[18px] font-semibold">
            {countLabel && <span className="sr-only">, </span>}
            {count}
            {countLabel && <span className="sr-only"> {countLabel}</span>}
          </span>
        )}
      </span>
    </div>
  )
}
