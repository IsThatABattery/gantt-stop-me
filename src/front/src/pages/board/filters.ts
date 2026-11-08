import type { Task } from '../../data/api/types'
import type { ColumnKey } from '../../data/types'

/** The three telemetry counters, each also a filter. */
export type CounterFilter = 'awaiting-approval' | 'needs-attention' | 'tests-running'

export interface BoardFilter {
  counter: CounterFilter | null
}

export const noFilter: BoardFilter = { counter: null }

export const isFiltered = (f: BoardFilter) => f.counter !== null

const counterMatches: Record<CounterFilter, (t: Task, column: ColumnKey) => boolean> = {
  'awaiting-approval': (_t, column) => column === 'user-check',
  'needs-attention': (t) => t.needsAttention !== null || t.runStatus === 'failed' || t.runStatus === 'error',
  'tests-running': (t) => t.runStatus === 'queued' || t.runStatus === 'running',
}

export function matches(t: Task, column: ColumnKey, f: BoardFilter) {
  return f.counter === null || counterMatches[f.counter](t, column)
}

/** How many tasks each counter would show. */
export function countAll(columns: { key: ColumnKey; tasks: Task[] }[]): Record<CounterFilter, number> {
  const count = (filter: CounterFilter) => columns.reduce((n, c) => n + c.tasks.filter((t) => counterMatches[filter](t, c.key)).length, 0)
  return {
    'awaiting-approval': count('awaiting-approval'),
    'needs-attention': count('needs-attention'),
    'tests-running': count('tests-running'),
  }
}
