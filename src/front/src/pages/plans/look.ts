import type { Tone } from '../../components/Chip'
import type { IconName } from '../../components/Icon'
import type { Plan, Proposal } from '../../data/views/plans'

export interface Look {
  tone: Tone
  icon: IconName
  word: string
  /** Classes for the icon, e.g. the moving packet while an agent works */
  iconClass?: string
}

const working = (word: string): Look => ({ tone: 'live', icon: 'running', iconClass: 'motion-safe:animate-live', word })

/** What an agent is doing on a plan right now, as a sentence. */
export const pendingText: Record<NonNullable<Plan['pending']>, string> = {
  ask: 'The interviewer is thinking of the next question…',
  draft: 'The interviewer is writing the plan doc…',
  generate: 'The task agent is proposing tasks…',
  revise: 'The task agent is revising a task…',
}

/**
 * A plan's state as a chip. Approve marks a plan waiting on a person (an answer, the doc, the tasks), live one an agent
 * is working on, signal one that broke.
 */
export function planLook(p: Plan): Look {
  if (p.error) return { tone: 'signal', icon: 'alert', word: 'Needs attention' }
  if (p.pending === 'ask') return working('Interviewer thinking')
  if (p.pending === 'draft') return working('Writing the doc')
  if (p.pending === 'generate') return working('Proposing tasks')
  if (p.pending === 'revise') return working('Revising a task')
  switch (p.status) {
    case 'briefing':
      return { tone: 'approve', icon: 'chat', word: 'Your answer' }
    case 'needs_review':
      return { tone: 'approve', icon: 'person', word: 'Doc to review' }
    case 'approved':
      return { tone: 'approve', icon: 'person', word: 'Tasks to review' }
    case 'done':
      return { tone: 'good', icon: 'check', word: 'Done' }
    case 'discarded':
      return { tone: 'neutral', icon: 'cancelled', word: 'Discarded' }
  }
}

export const proposalLook: Record<Proposal['status'], Look> = {
  needs_review: { tone: 'approve', icon: 'person', word: 'Needs review' },
  approved: { tone: 'good', icon: 'check', word: 'Approved' },
  approved_edited: { tone: 'good', icon: 'check', word: 'Approved, edited' },
  sent_back: { tone: 'neutral', icon: 'agent', word: 'With the agent' },
  dropped: { tone: 'neutral', icon: 'cancelled', word: 'Dropped' },
}
