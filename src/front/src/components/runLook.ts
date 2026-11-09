import type { Tone } from './Chip'
import type { IconName } from './Icon'
import type { RunStatus } from '../data/api/types'

/**
 * Chip tone and icon for a task's latest test run. Every colour pairs with an icon and a word.
 * Running is live, and its packet moves only when the user allows motion. A failed run (the code is wrong) and a
 * runner error (the run broke) both need a person, so both take signal, with different marks.
 */
export const runLook: Record<RunStatus, { tone: Tone; icon?: IconName; iconClass?: string }> = {
  queued: { tone: 'neutral', icon: 'queued' },
  running: { tone: 'live', icon: 'running', iconClass: 'motion-safe:animate-live' },
  passed: { tone: 'good', icon: 'check' },
  failed: { tone: 'signal', icon: 'failed' },
  error: { tone: 'signal', icon: 'error' },
  cancelled: { tone: 'neutral', icon: 'cancelled' },
}
