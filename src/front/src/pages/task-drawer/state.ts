import type { RunStatus, Task, TaskDetail } from '../../data/api/types'
import type { ColumnKey } from '../../data/types'
import { useRun, useRuns } from '../../data/views/runs'
import { columnLabels } from '../../labels/labels'

/** The column's word, with the plain one beside it when a themed label set gives it another. */
export function columnName(column: ColumnKey) {
  const { name, plain } = columnLabels[column]
  return name === plain ? name : `${name} · ${plain}`
}

/** What the task waits on a person for, if anything: the same signal and approve the board's tabs light with. */
export function personNeeded(task: Task, column?: ColumnKey): 'signal' | 'approve' | null {
  if (task.needsAttention || task.runStatus === 'failed' || task.runStatus === 'error') return 'signal'
  if (column === 'user-check' && task.state === 'open') return 'approve'
  return null
}

/** How many tests the Tests tab lists: the latest run's results once it has them, else the issue's tests. */
export function useTestCount(projectId: string, task: TaskDetail) {
  const latest = useRuns(projectId, String(task.id)).data?.[0]
  const run = useRun(projectId, latest?.id)
  return run.data?.results.length || task.tests.length
}

export const isLive = (s?: RunStatus) => s === 'queued' || s === 'running'
