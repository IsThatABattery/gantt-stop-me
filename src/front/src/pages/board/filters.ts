import type { CounterTone } from '../../components/Counter'
import type { Task } from '../../data/api/types'
import type { ColumnKey } from '../../data/types'
import { labels } from '../../labels/labels'

/** The three header counters, each also a filter. */
export type CounterFilter = 'awaiting-approval' | 'needs-attention' | 'tests-running'

export interface BoardFilter {
  counter: CounterFilter | null
}

export const noFilter: BoardFilter = { counter: null }

export const isFiltered = (f: BoardFilter) => f.counter !== null

/** The column where a person checks a task's results and approves it */
export const APPROVAL_COLUMN: ColumnKey = 'user-check'

/** A task a person has to look at: a runner error flagged it, or its tests failed. */
export const needsPerson = (t: Task) => t.needsAttention !== null || t.runStatus === 'failed' || t.runStatus === 'error'

const counterMatches: Record<CounterFilter, (t: Task, column: ColumnKey) => boolean> = {
  'awaiting-approval': (_t, column) => column === APPROVAL_COLUMN,
  'needs-attention': needsPerson,
  'tests-running': (t) => t.runStatus === 'queued' || t.runStatus === 'running',
}

/** In header order: what waits on a person first, then what is moving. */
export const counters: { filter: CounterFilter; tone: CounterTone; label: string }[] = [
  { filter: 'awaiting-approval', tone: 'approve', label: labels.awaitingApproval },
  { filter: 'needs-attention', tone: 'signal', label: labels.needsAttention },
  { filter: 'tests-running', tone: 'live', label: labels.testsRunning },
]

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

/** "1 task", "4 tasks", "2 of 4 tasks" */
export function taskCount(n: number, of?: number) {
  const word = (of ?? n) === 1 ? labels.task : labels.tasks
  return `${of === undefined ? n : `${n} of ${of}`} ${word.toLowerCase()}`
}
