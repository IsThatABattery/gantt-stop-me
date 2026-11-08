import type { ID, Task } from '../../data/types'

/** The three telemetry counters, each also a filter. */
export type CounterFilter = 'awaiting-approval' | 'blocked' | 'agents-active'

export interface BoardFilter {
  /** Objective; null shows every objective */
  featureId: ID | null
  counter: CounterFilter | null
}

export const noFilter: BoardFilter = { featureId: null, counter: null }

export const isFiltered = (f: BoardFilter) => f.featureId !== null || f.counter !== null

const counterMatches: Record<CounterFilter, (t: Task) => boolean> = {
  'awaiting-approval': (t) => t.column === 'user-check' && t.status !== 'landed',
  blocked: (t) => t.status === 'blocked',
  'agents-active': (t) => t.assignee?.kind === 'agent' && t.status !== 'landed',
}

export function matches(t: Task, f: BoardFilter) {
  if (f.featureId !== null && t.featureId !== f.featureId) return false
  if (f.counter !== null && !counterMatches[f.counter](t)) return false
  return true
}
