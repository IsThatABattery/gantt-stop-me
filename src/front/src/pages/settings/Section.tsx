import { useId, type ElementType, type ReactNode } from 'react'
import { Icon } from '../../components/Icon'
import { InkTab, Tile } from '../../components/Tile'
import { cx } from '../../lib/cx'

interface SectionProps {
  id?: string
  title: string
  /** The purpose line under the tab, as each stage on the board has one */
  intro?: ReactNode
  /** The count box on the tab, e.g. how many providers there are */
  count?: number
  countLabel?: string
  /** Lights the tab when the section needs the person, e.g. GitHub isn't linked. Give it a markLabel. */
  mark?: 'signal' | 'approve'
  markLabel?: string
  children: ReactNode
}

/**
 * One settings section, in the board's language: a gray tile under a solid ink tab, its purpose line, then its
 * contents, usually white cards. The tab is the section's h2, so the region is named by it.
 */
export function Section({ id, title, intro, count, countLabel, mark, markLabel, children }: SectionProps) {
  const headingId = useId()
  return (
    <Tile
      as="section"
      id={id}
      aria-labelledby={headingId}
      className="scroll-mt-5"
      frameClassName="gap-3"
      tab={
        <InkTab id={headingId} count={count} countLabel={countLabel} mark={mark} markLabel={markLabel}>
          {title}
        </InkTab>
      }
    >
      {intro && <p className="m-0 max-w-[70ch] px-0.5 text-[13px] leading-[1.45] text-ink-2">{intro}</p>}
      {children}
    </Tile>
  )
}

interface CardProps {
  children: ReactNode
  as?: ElementType
  className?: string
  /** The space between the card's children; gap-4 by default */
  gap?: string
  /** Tighter padding, for small cards in a list such as the workflows */
  compact?: boolean
  /** More classes for the inner frame. Padding given as px- or py- replaces the card's own. */
  frameClassName?: string
  id?: string
}

/** A white keylined card, as the board's tasks: a 1px line edge, then an inset keyline frame around the content. */
export function Card({ children, as: As = 'div', className = '', gap = 'gap-4', compact = false, frameClassName = '', id }: CardProps) {
  const padding = compact ? 'px-3 py-2.5' : 'px-4 py-4 max-sm:px-3 max-sm:py-3'
  return (
    <As id={id} className={cx('min-w-0 rounded-card border border-line bg-paper p-[3px]', className)}>
      <div className={cx(`flex h-full min-w-0 flex-col rounded-[2px] border border-keyline ${gap} ${padding}`, frameClassName)}>{children}</div>
    </As>
  )
}

/**
 * A question that needs an answer before anything happens, in place: an approve mark, the question, then the
 * buttons. The same look as the unsaved-changes guard.
 */
export function Confirm({ id, question, children }: { id: string; question: ReactNode; children: ReactNode }) {
  return (
    <div role="alertdialog" aria-labelledby={id} className="flex flex-wrap items-center gap-3 rounded-card border-[1.5px] border-ink bg-paper p-3">
      <span aria-hidden="true" className="grid size-9 flex-none place-items-center rounded-[2px] bg-approve text-on-approve">
        <Icon name="alert" size={18} />
      </span>
      <p id={id} className="m-0 min-w-[12rem] flex-1 text-sm font-semibold">
        {question}
      </p>
      <div className="flex flex-wrap gap-2.5">{children}</div>
    </div>
  )
}

