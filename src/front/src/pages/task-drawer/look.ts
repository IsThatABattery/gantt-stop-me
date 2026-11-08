import type { Tone } from '../../components/Chip'
import type { IconName } from '../../components/Icon'
import type { TaskStatus, TraceKind } from '../../data/types'
import { labels } from '../../labels/labels'

// Status chip in the header: the column's space name, then a plain word
// (PayloadDetail.dc.html shows "Docking · Awaiting review").
export const plainStatus: Record<TaskStatus, string> = {
  queued: 'Queued',
  ready: 'Ready',
  working: 'Working',
  hail: labels.waitingOnYou,
  blocked: 'Blocked',
  'awaiting-approval': labels.awaitingReview,
  'in-review': 'In review',
  landed: 'Done',
}

/** Same color meaning as the board cards (AGENT_BRIEF §5, §9). */
export function statusLook(status: TaskStatus, byAgent: boolean): { tone: Tone; icon?: IconName } {
  switch (status) {
    case 'queued':
      return { tone: 'neutral' }
    case 'ready':
      return { tone: 'ion' }
    case 'working':
      return byAgent ? { tone: 'astronaut', icon: 'orbit' } : { tone: 'ion', icon: 'orbit' }
    case 'hail':
      return { tone: 'warning', icon: 'chat' }
    case 'blocked':
      return { tone: 'critical', icon: 'alert' }
    case 'awaiting-approval':
    case 'in-review':
      return { tone: 'warning', icon: 'dock' }
    case 'landed':
      return { tone: 'good', icon: 'check' }
  }
}

/** Trace step color: violet for the astronaut's own steps, cyan for results, amber when it needs a human. */
export const traceTone: Record<TraceKind, string> = {
  plan: 'border-astronaut text-astronaut',
  read: 'border-astronaut text-astronaut',
  edit: 'border-astronaut text-astronaut',
  run: 'border-astronaut text-astronaut',
  result: 'border-ion text-ion',
  hail: 'border-warning text-warning',
  docking: 'border-warning text-warning',
}

/** "14:12" in local time. */
export const clockTime = (iso: string) =>
  new Date(iso).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
