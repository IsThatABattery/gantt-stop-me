// Mission control board (feature/frontend-board).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { columnLabels } from '../../labels/labels'
import { keys } from '../keys'
import { ConflictError, db, delay, mustFind, newId, NotFoundError, signedInUserId } from '../mock/store'
import { COLUMN_ORDER, type Board, type ColumnKey, type ID, type Task } from '../types'

export function getBoard(projectId: ID): Promise<Board> {
  const project = db.projects.find((p) => p.id === projectId)
  if (!project) return Promise.reject(new NotFoundError('Project'))
  const tasks = db.tasks.filter((t) => t.projectId === projectId).sort((a, b) => a.position - b.position)
  return delay({
    project,
    sprint: db.sprints.find((s) => s.id === project.activeSprintId) ?? null,
    columns: COLUMN_ORDER.map((key) => ({ key, tasks: tasks.filter((t) => t.column === key) })),
    epics: db.epics.filter((e) => e.projectId === projectId),
    features: db.features.filter((f) => f.projectId === projectId),
    users: db.users,
    agents: db.agents,
  })
}

export const useBoard = (projectId: ID) => useQuery({ queryKey: keys.board(projectId), queryFn: () => getBoard(projectId) })

export interface MoveTarget {
  column: ColumnKey
  /** The card it lands above; null puts it at the bottom of the column */
  beforeTaskId: ID | null
  /** The task's version when the move started (ARCHITECTURE.md "Tasks and moves") */
  version: number
}

export interface MoveVars extends MoveTarget {
  taskId: ID
}

/** Smallest gap between neighbours before a column is renumbered. */
const MIN_GAP = 1e-6

let conflictsToForce = 0

/**
 * Make the next move rejected with a conflict, as if someone else moved the
 * card first. Tests call it directly; in dev it is also on
 * `window.forceMoveConflict()` for trying it in the browser.
 */
export function forceMoveConflict(times = 1) {
  conflictsToForce = times
}

if (import.meta.env.DEV && typeof window !== 'undefined') {
  Object.assign(window, { forceMoveConflict })
}

/** Status a task gets on entering a column, if the column decides it. */
function statusOnEnter(task: Task, column: ColumnKey): Task['status'] {
  if (column === 'closed') return 'landed'
  // Reopened: back to the queue, like a reopened GitHub issue.
  if (task.column === 'closed') return 'queued'
  return task.status
}

export function moveTask(projectId: ID, taskId: ID, { column, beforeTaskId, version }: MoveTarget): Promise<Task> {
  try {
    const task = mustFind(db.tasks, (t) => t.id === taskId && t.projectId === projectId, 'Task')
    if (conflictsToForce > 0) {
      conflictsToForce--
      throw new ConflictError(`${task.key} was changed by someone else first`)
    }
    if (version !== task.version) throw new ConflictError(`${task.key} was changed by someone else first`)

    const siblings = () =>
      db.tasks
        .filter((t) => t.projectId === projectId && t.column === column && t.id !== taskId)
        .sort((a, b) => a.position - b.position)

    let position = placeBetween(siblings(), beforeTaskId)
    if (position === null) {
      siblings().forEach((t, i) => (t.position = i + 1))
      position = placeBetween(siblings(), beforeTaskId)!
    }

    const from = task.column
    task.status = statusOnEnter(task, column)
    task.column = column
    task.position = position
    task.version++

    db.activityEvents.unshift({
      id: newId('ev'),
      projectId,
      at: new Date().toISOString(),
      actor: { kind: 'user', id: signedInUserId },
      verb: 'moved',
      taskId,
      detail:
        from === column
          ? `Reordered in ${columnLabels[column].name}`
          : `Moved from ${columnLabels[from].name} to ${columnLabels[column].name}`,
      needsAction: false,
    })
    return delay(task)
  } catch (e) {
    // Rejections take as long as a real round trip, so the snap-back is visible.
    return new Promise((_, reject) => setTimeout(() => reject(e), 150))
  }
}

/**
 * Midpoint position above `beforeTaskId` in a sorted column, or null when the
 * gap is too small and the column needs renumbering.
 */
function placeBetween(sorted: Task[], beforeTaskId: ID | null): number | null {
  const i = beforeTaskId === null ? -1 : sorted.findIndex((t) => t.id === beforeTaskId)
  if (i === -1) return sorted.length ? sorted[sorted.length - 1].position + 1 : 1
  const next = sorted[i].position
  const prev = i > 0 ? sorted[i - 1].position : next - 2
  if (next - prev < MIN_GAP) return null
  return (prev + next) / 2
}

/**
 * The board with one task moved, as the API will save it. Used for the
 * optimistic update, so the card moves before the request returns.
 */
export function applyMove(board: Board, { taskId, column, beforeTaskId }: MoveVars): Board {
  const task = board.columns.flatMap((c) => c.tasks).find((t) => t.id === taskId)
  if (!task) return board
  const moved: Task = { ...task, column, status: statusOnEnter(task, column), version: task.version + 1 }
  return {
    ...board,
    columns: board.columns.map((c) => {
      const tasks = c.tasks.filter((t) => t.id !== taskId)
      if (c.key !== column) return { ...c, tasks }
      const at = beforeTaskId === null ? -1 : tasks.findIndex((t) => t.id === beforeTaskId)
      tasks.splice(at === -1 ? tasks.length : at, 0, moved)
      return { ...c, tasks }
    }),
  }
}

/**
 * Move a card optimistically. The board cache changes at once; if the API
 * rejects the move the cache goes back to what it was and `onRejected` runs
 * with the task and the error, so the board can say why.
 */
export function useMoveTask(projectId: ID, onRejected: (task: Task, error: Error) => void) {
  const client = useQueryClient()
  const boardKey = keys.board(projectId)
  return useMutation({
    mutationFn: ({ taskId, ...target }: MoveVars) => moveTask(projectId, taskId, target),
    onMutate: async (vars) => {
      await client.cancelQueries({ queryKey: boardKey })
      const before = client.getQueryData<Board>(boardKey)
      if (before) client.setQueryData(boardKey, applyMove(before, vars))
      return { before }
    },
    onError: (error, vars, context) => {
      const task = context?.before?.columns.flatMap((c) => c.tasks).find((t) => t.id === vars.taskId)
      if (context?.before) client.setQueryData(boardKey, context.before)
      if (task) onRejected(task, error)
    },
    onSettled: () =>
      Promise.all([
        client.invalidateQueries({ queryKey: boardKey }),
        client.invalidateQueries({ queryKey: keys.activity(projectId) }),
      ]),
  })
}
