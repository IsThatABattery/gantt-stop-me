import type { Tone } from '../../components/Chip'
import type { IconName } from '../../components/Icon'
import type { TaskStatus, TraceKind } from '../../data/types'
import { labels } from '../../labels/labels'

// Status chip in the header for the mock-data drawer: a plain word per task status.
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
      return { tone: 'live' }
    case 'working':
      return byAgent ? { tone: 'live', icon: 'agent' } : { tone: 'live', icon: 'running' }
    case 'hail':
      return { tone: 'approve', icon: 'chat' }
    case 'blocked':
      return { tone: 'signal', icon: 'alert' }
    case 'awaiting-approval':
    case 'in-review':
      return { tone: 'approve', icon: 'person' }
    case 'landed':
      return { tone: 'good', icon: 'check' }
  }
}

/** Trace step colour: the accent for an agent's own steps and results, approve when it needs a person. */
export const traceTone: Record<TraceKind, string> = {
  plan: 'border-accent text-accent',
  read: 'border-accent text-accent',
  edit: 'border-accent text-accent',
  run: 'border-accent text-accent',
  result: 'border-accent text-accent',
  hail: 'border-approve text-approve-ink',
  docking: 'border-approve text-approve-ink',
}

/** "14:12" in local time. */
export const clockTime = (iso: string) =>
  new Date(iso).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
