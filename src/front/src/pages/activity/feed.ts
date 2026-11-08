import type { ActivityEvent } from '../../data/types'
import { labels } from '../../labels/labels'

export type ActivityFilter = 'all' | 'users' | 'agents' | 'needs-action'

export const filterLabels: Record<ActivityFilter, string> = {
  all: 'All',
  users: labels.operatorsOnly,
  agents: labels.agentsOnly,
  'needs-action': labels.needsAction,
}

export function filterEvents(events: ActivityEvent[], filter: ActivityFilter) {
  switch (filter) {
    case 'all':
      return events
    case 'users':
      return events.filter((e) => e.actor.kind === 'user')
    case 'agents':
      return events.filter((e) => e.actor.kind === 'agent')
    case 'needs-action':
      return events.filter((e) => e.needsAction)
  }
}

/** "14:12" in local time. */
export const clockTime = (iso: string) => new Date(iso).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })

const localDay = (d: Date) => `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`

export function dayLabel(iso: string, now = Date.now()) {
  const d = new Date(iso)
  const today = new Date(now)
  const yesterday = new Date(now)
  yesterday.setDate(today.getDate() - 1)
  if (localDay(d) === localDay(today)) return 'Today'
  if (localDay(d) === localDay(yesterday)) return 'Yesterday'
  return d.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' })
}

/** Consecutive runs of same-day events; input is newest first. */
export function groupByDay(events: ActivityEvent[], now: number) {
  const groups: { key: string; label: string; events: ActivityEvent[] }[] = []
  for (const e of events) {
    const key = localDay(new Date(e.at))
    const last = groups.at(-1)
    if (last?.key === key) last.events.push(e)
    else groups.push({ key, label: dayLabel(e.at, now), events: [e] })
  }
  return groups
}
