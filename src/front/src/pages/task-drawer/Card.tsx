import type { ElementType, ReactNode } from 'react'
import { cx } from '../../lib/cx'

/**
 * plain: a white card with a 1px line edge and the inset keyline, as tasks are on the board.
 * signal: edge and keyline in signal, for something that failed or broke.
 * approve: a 2px approve keyline, for something waiting on a person's check.
 */
export type CardTone = 'plain' | 'signal' | 'approve'

const frames: Record<CardTone, string> = {
  plain: 'border border-keyline p-3',
  signal: 'border border-signal p-3',
  approve: 'border-2 border-approve p-[11px]',
}

interface CardProps {
  tone?: CardTone
  as?: ElementType
  className?: string
  /** Classes for the inner frame; it is a flex column with a 8px gap */
  frameClassName?: string
  children: ReactNode
}

/** B's task card: paper, a 1px line edge, 3px of paper, then an inset keyline frame around the content. */
export function Card({ tone = 'plain', as: As = 'div', className = '', frameClassName = '', children }: CardProps) {
  return (
    <As className={cx(`min-w-0 rounded-card border bg-paper p-[3px] text-ink ${tone === 'signal' ? 'border-signal' : 'border-line'}`, className)}>
      <div className={`flex min-w-0 flex-col gap-2 rounded-[2px] ${frames[tone]} ${frameClassName}`}>{children}</div>
    </As>
  )
}
