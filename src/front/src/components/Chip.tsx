import type { ReactNode } from 'react'
import { cx } from '../lib/cx'
import { Icon, type IconName } from './Icon'

/**
 * neutral: a gray block, for columns, tags and queued or cancelled runs.
 * live: what is happening now (tests running), in the accent.
 * signal: a person is needed, because something failed or needs attention.
 * approve: waiting for a person's approval.
 * good: success, a gray block whose icon carries the colour.
 */
export type Tone = 'neutral' | 'live' | 'signal' | 'approve' | 'good'

/** Solid fills: the board's chips. Text on each fill is its on- colour, checked for contrast in every theme. */
const solid: Record<Tone, string> = {
  neutral: 'border-transparent bg-block text-ink',
  live: 'border-transparent bg-accent text-on-accent',
  signal: 'border-transparent bg-signal text-on-signal',
  approve: 'border-transparent bg-approve text-on-approve',
  good: 'border-transparent bg-block text-ink [&>svg]:text-good',
}

/** Soft: quieter, for chips that sit beside a louder one or in running text. */
const soft: Record<Tone, string> = {
  neutral: 'bg-transparent text-ink-2 border-line',
  live: 'bg-transparent text-accent border-accent',
  signal: 'border-transparent bg-signal-wash text-signal-ink',
  approve: 'border-transparent bg-approve/20 text-ink',
  good: 'bg-transparent text-good border-good',
}

interface ChipProps {
  tone: Tone
  /** The word. State is never colour alone: always pass a word, and an icon where the meaning matters. */
  children: ReactNode
  /** Drawn before the word at 14px */
  icon?: IconName
  /** Classes for the icon, e.g. motion-safe:animate-live while tests run */
  iconClassName?: string
  /** A quieter look: an outline, or a wash for signal and approve */
  soft?: boolean
  /** Both are 2px-radius blocks; tag is for counts and labels, at medium weight. */
  shape?: 'pill' | 'tag'
  className?: string
}

/** A small 24px block: an icon slot and a word, filled by tone. */
export function Chip({ tone, children, icon, iconClassName, soft: quiet = false, shape = 'pill', className = '' }: ChipProps) {
  return (
    <span
      className={cx(
        `inline-flex h-6 max-w-full items-center gap-[5px] rounded-[2px] border px-2 text-xs leading-none whitespace-nowrap ${
          shape === 'tag' ? 'font-medium' : 'font-semibold'
        } ${(quiet ? soft : solid)[tone]}`,
        className,
      )}
    >
      {icon && <Icon name={icon} size={14} className={cx('-ml-0.5 flex-none', iconClassName)} />}
      {children}
    </span>
  )
}
