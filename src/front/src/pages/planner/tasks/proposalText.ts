import type { Tone } from '../../../components/Chip'
import type { Assignee, DocSection, ProposalStatus } from '../../../data/types'
import type { ReviewContext } from '../../../data/views/plannerTasks'
import { autonomyLabels, labels } from '../../../labels/labels'

export const statusTone: Record<ProposalStatus, Tone> = {
  'needs-review': 'ion',
  revised: 'ion',
  approved: 'good',
  'approved-edited': 'good',
  'sent-back': 'warning',
  dropped: 'neutral',
}

/** "Astronaut-2 · Guided", "MK · Operator" or "Unassigned". */
export function assigneeText(a: Assignee | null, ctx: ReviewContext | undefined) {
  if (!a) return labels.unassigned
  if (a.kind === 'agent') {
    const agent = ctx?.agents.find((x) => x.id === a.id)
    return agent ? `${agent.name} · ${autonomyLabels[agent.autonomy]}` : labels.agent
  }
  const user = ctx?.users.find((x) => x.id === a.id)
  return user ? `${user.initials} · ${labels.user}` : labels.user
}

/**
 * The doc text a proposal came from. `source` reads like "Success criteria,
 * item 2"; returns null when the section or item is not in the doc.
 */
export function sourceText(source: string, sections: DocSection[] | undefined): string | null {
  const match = /^(.*?)(?:,\s*item\s+(\d+))?$/i.exec(source.trim())
  if (!match || !sections) return null
  const [, title, item] = match
  const section = sections.find((s) => s.title.toLowerCase() === title.trim().toLowerCase())
  if (!section) return null
  if (Array.isArray(section.body)) return item ? (section.body[Number(item) - 1] ?? null) : section.body.join(' ')
  return section.body || null
}
