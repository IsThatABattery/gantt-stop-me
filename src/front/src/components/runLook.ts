import type { Tone } from './Chip'
import type { IconName } from './Icon'
import type { RunStatus } from '../data/api/types'

/**
 * Chip tone and icon for a task's latest test run. Every color pairs with a word.
 * A queued run waits on the pad; a running one lifts off, moving only when the user allows motion.
 */
export const runLook: Record<RunStatus, { tone: Tone; icon?: IconName; iconClass?: string }> = {
  queued: { tone: 'neutral', icon: 'launchpad' },
  running: { tone: 'ion', icon: 'liftoff', iconClass: 'motion-safe:animate-liftoff' },
  passed: { tone: 'good', icon: 'check' },
  failed: { tone: 'critical', icon: 'alert' },
  error: { tone: 'critical', icon: 'alert' },
  cancelled: { tone: 'neutral' },
}
