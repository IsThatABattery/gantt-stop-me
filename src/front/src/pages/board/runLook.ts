import type { Tone } from '../../components/Chip'
import type { IconName } from '../../components/Icon'
import type { RunStatus } from '../../data/api/types'

/** Chip tone and icon for a task's latest test run. Every color pairs with a word. */
export const runLook: Record<RunStatus, { tone: Tone; icon?: IconName }> = {
  queued: { tone: 'neutral' },
  running: { tone: 'ion', icon: 'orbit' },
  passed: { tone: 'good', icon: 'check' },
  failed: { tone: 'critical', icon: 'alert' },
  error: { tone: 'critical', icon: 'alert' },
  cancelled: { tone: 'neutral' },
}
