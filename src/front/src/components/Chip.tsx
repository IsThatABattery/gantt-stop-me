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
  /** pill (999px) for status, tag (8px) for counts and labels */
  shape?: 'pill' | 'tag'
  className?: string
}

export function Chip({ tone, children, shape = 'pill', className = '' }: ChipProps) {
  const shapeClass = shape === 'pill' ? 'rounded-full px-2 py-0.5' : 'rounded-lg px-2.5 py-[3px]'
  return (
    <span className={cx(`inline-flex items-center gap-1.5 border text-xs leading-[18px] whitespace-nowrap ${shapeClass} ${toneText[tone]}`, className)}>
      {children}
    </span>
  )
}
