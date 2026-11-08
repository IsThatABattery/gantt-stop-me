import type { ReactNode } from 'react'
import { cx } from '../lib/cx'

export type Tone = 'neutral' | 'ion' | 'astronaut' | 'warning' | 'critical' | 'good'

// Text color and border per tone. State is never color-only: callers always
// pass a word, and an icon where the meaning matters.
const toneText: Record<Tone, string> = {
  neutral: 'text-text-muted border-text-muted',
  ion: 'text-ion border-ion',
  astronaut: 'text-astronaut-text border-astronaut',
  warning: 'text-warning border-warning',
  critical: 'text-critical border-critical',
  good: 'text-good border-good',
}

interface ChipProps {
  tone: Tone
  children: ReactNode
  /** pill for status, tag for counts and labels; both 4px corners, the tag a little wider */
  shape?: 'pill' | 'tag'
  className?: string
}

export function Chip({ tone, children, shape = 'pill', className = '' }: ChipProps) {
  const shapeClass = shape === 'pill' ? 'rounded-control px-2 py-1' : 'rounded-control px-3 py-1'
  return (
    <span className={cx(`inline-flex items-center gap-2 border text-xs leading-[18px] whitespace-nowrap ${shapeClass} ${toneText[tone]}`, className)}>
      {children}
    </span>
  )
}
