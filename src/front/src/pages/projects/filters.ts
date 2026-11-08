import type { ID, ProjectSummary } from '../../data/types'

export const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

export const FILTERS = [
  { key: 'all', label: 'All' },
  { key: 'mine', label: 'Mine' },
  { key: 'favorites', label: 'Favorites' },
  { key: 'archived', label: 'Archived' },
] as const
export type Filter = (typeof FILTERS)[number]['key']

export const SORTS = [
  { key: 'recent', label: 'Recent activity' },
  { key: 'name', label: 'Name' },
  { key: 'window-end', label: 'Launch window end' },
] as const
export type Sort = (typeof SORTS)[number]['key']

interface View {
  filter: Filter
  query: string
  sort: Sort
  userId: ID | undefined
  favorites: ID[]
}

function matchesFilter(p: ProjectSummary, { filter, userId, favorites }: View) {
  if (filter === 'archived') return p.archived
  if (p.archived) return false
  if (filter === 'mine') return !!userId && p.memberIds.includes(userId)
  if (filter === 'favorites') return favorites.includes(p.id)
  return true
}

function matchesQuery(p: ProjectSummary, query: string) {
  const q = query.trim().toLowerCase()
  return !q || p.name.toLowerCase().includes(q) || p.description.toLowerCase().includes(q)
}

const compare: Record<Sort, (a: ProjectSummary, b: ProjectSummary) => number> = {
  recent: (a, b) => b.lastActivityAt.localeCompare(a.lastActivityAt),
  name: (a, b) => a.name.localeCompare(b.name),
  // Missions with no active launch window go last.
  'window-end': (a, b) => {
    if (!a.activeSprint || !b.activeSprint) return Number(!a.activeSprint) - Number(!b.activeSprint)
    return a.activeSprint.end.localeCompare(b.activeSprint.end)
  },
}

export function visibleProjects(projects: ProjectSummary[], view: View) {
  return projects.filter((p) => matchesFilter(p, view) && matchesQuery(p, view.query)).sort(compare[view.sort])
}

/** Totals for the Needs you strip, and the mission that needs attention most. */
export function needsYou(projects: ProjectSummary[]) {
  const live = projects.filter((p) => !p.archived)
  const score = (p: ProjectSummary) => p.stats.awaitingApproval + p.stats.hails + p.stats.blocked
  const top = live.reduce<ProjectSummary | null>((best, p) => (score(p) > (best ? score(best) : 0) ? p : best), null)
  return {
    approval: live.reduce((n, p) => n + p.stats.awaitingApproval, 0),
    hails: live.reduce((n, p) => n + p.stats.hails, 0),
    blockedProjects: live.filter((p) => p.stats.blocked > 0).length,
    top,
  }
}
