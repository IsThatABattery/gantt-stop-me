// Mission control board: the project's five columns and their tasks, from
// GET /api/projects/{id}/board, and optimistic moves (docs/board.md).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { BoardResponse, Column, Project, Task } from '../api/types'
import { ConflictError } from '../errors'
import { keys } from '../keys'

export interface BoardColumn extends Column {
  /** In board order */
  tasks: Task[]
}

export interface Board {
  project: Project
  /** In board order */
  columns: BoardColumn[]
}

/** The API's flat task list, grouped under its columns. The API already sorts tasks. */
export function toBoard({ project, columns, tasks }: BoardResponse): Board {
  return {
    project,
    columns: [...columns]
      .sort((a, b) => a.position - b.position)
      .map((c) => ({ ...c, tasks: tasks.filter((t) => t.columnId === c.id) })),
  }
}

export const getBoard = (projectId: string) =>
  api<BoardResponse>('GET', `/projects/${projectId}/board`).then(toBoard)

export const useBoard = (projectId: string) =>
  useQuery({ queryKey: keys.board(projectId), queryFn: () => getBoard(projectId) })

export interface MoveTarget {
  columnId: number
  /** The card it lands above; null puts it at the bottom of the column */
  beforeTaskId: number | null
  /** The task's version when the move started, so a stale move is refused */
  version: number
}

export interface MoveVars extends MoveTarget {
  taskId: number
}

export const moveTask = (projectId: string, taskId: number, target: MoveTarget) =>
  api<Task>('POST', `/projects/${projectId}/tasks/${taskId}/move`, target)

/** The board with `task` taken out and put in `columnId` above `beforeTaskId`. */
function place(board: Board, task: Task, beforeTaskId: number | null): Board {
  return {
    ...board,
    columns: board.columns.map((c) => {
      const tasks = c.tasks.filter((t) => t.id !== task.id)
      if (c.id !== task.columnId) return { ...c, tasks }
      const at = beforeTaskId === null ? -1 : tasks.findIndex((t) => t.id === beforeTaskId)
      tasks.splice(at === -1 ? tasks.length : at, 0, task)
      return { ...c, tasks }
    }),
  }
}

/**
 * The board with one task moved, as the API will save it. Used for the
 * optimistic update, so the card moves before the request returns.
 */
export function applyMove(board: Board, { taskId, columnId, beforeTaskId }: MoveVars): Board {
  const task = board.columns.flatMap((c) => c.tasks).find((t) => t.id === taskId)
  const column = board.columns.find((c) => c.id === columnId)
  if (!task || !column) return board
  const moved: Task = { ...task, columnId, state: column.closesIssue ? 'closed' : 'open', version: task.version + 1 }
  return place(board, moved, beforeTaskId)
}

/**
 * The board with the API's current copy of a task, placed by its position.
 * A 409 conflict answers it, so the card lands where it really is.
 */
export function withCurrentTask(board: Board, task: Task): Board {
  const column = board.columns.find((c) => c.id === task.columnId)
  const before = column?.tasks.find((t) => t.id !== task.id && t.position > task.position) ?? null
  return place(board, task, before?.id ?? null)
}

/**
 * Move a card optimistically. The board cache changes at once; if the API
 * rejects the move the cache goes back to what it was and `onRejected` runs
 * with the task where it now is and the error, so the board can say why.
 */
export function useMoveTask(projectId: string, onRejected: (task: Task, error: Error) => void) {
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
      const before = context?.before
      const task = before?.columns.flatMap((c) => c.tasks).find((t) => t.id === vars.taskId)
      const current = error instanceof ConflictError ? (error.body as { task?: Task } | null)?.task : undefined
      if (before) client.setQueryData(boardKey, current ? withCurrentTask(before, current) : before)
      if (task) onRejected(current ?? task, error)
    },
    onSettled: () => client.invalidateQueries({ queryKey: boardKey }),
  })
}
