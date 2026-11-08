import { describe, expect, it } from 'vitest'
import type { BoardResponse, Task } from '../api/types'
import { applyMove, toBoard, withCurrentTask } from './board'

const task = (id: number, columnId: number, position: number, extra: Partial<Task> = {}): Task => ({
  id,
  issueNumber: id + 100,
  columnId,
  position,
  version: 1,
  title: `Task ${id}`,
  state: columnId === 3 ? 'closed' : 'open',
  url: '',
  runStatus: null,
  needsAttention: null,
  assignees: [],
  ...extra,
})

const response: BoardResponse = {
  project: { id: 1, owner: 'ada', name: 'gantt', permission: 'write' },
  // Out of order on purpose: the board sorts columns by position.
  columns: [
    { id: 3, key: 'closed', name: 'Closed', position: 2, closesIssue: true },
    { id: 1, key: 'backlog', name: 'Backlog', position: 0, closesIssue: false },
    { id: 2, key: 'in-progress', name: 'In Progress', position: 1, closesIssue: false },
  ],
  tasks: [task(1, 1, 1), task(2, 1, 2), task(3, 2, 1), task(4, 3, 1)],
}

const ids = (board: ReturnType<typeof toBoard>) => Object.fromEntries(board.columns.map((c) => [c.key, c.tasks.map((t) => t.id)]))

describe('toBoard', () => {
  it('groups tasks under their columns, in column order', () => {
    const board = toBoard(response)
    expect(board.columns.map((c) => c.key)).toEqual(['backlog', 'in-progress', 'closed'])
    expect(ids(board)).toEqual({ backlog: [1, 2], 'in-progress': [3], closed: [4] })
  })
})

describe('applyMove', () => {
  it('puts the card above beforeTaskId and bumps its version', () => {
    const board = applyMove(toBoard(response), { taskId: 2, columnId: 2, beforeTaskId: 3, version: 1 })
    expect(ids(board)).toEqual({ backlog: [1], 'in-progress': [2, 3], closed: [4] })
    expect(board.columns[1].tasks[0]).toMatchObject({ columnId: 2, version: 2 })
  })

  it('puts the card at the bottom when beforeTaskId is null', () => {
    const board = applyMove(toBoard(response), { taskId: 1, columnId: 1, beforeTaskId: null, version: 1 })
    expect(ids(board).backlog).toEqual([2, 1])
  })

  it('closes the issue in a closing column and reopens it on the way out', () => {
    const closed = applyMove(toBoard(response), { taskId: 1, columnId: 3, beforeTaskId: null, version: 1 })
    expect(closed.columns[2].tasks.find((t) => t.id === 1)?.state).toBe('closed')
    const reopened = applyMove(closed, { taskId: 4, columnId: 1, beforeTaskId: null, version: 1 })
    expect(reopened.columns[0].tasks.find((t) => t.id === 4)?.state).toBe('open')
  })

  it('leaves the board alone for an unknown task or column', () => {
    const board = toBoard(response)
    expect(applyMove(board, { taskId: 99, columnId: 1, beforeTaskId: null, version: 1 })).toBe(board)
    expect(applyMove(board, { taskId: 1, columnId: 99, beforeTaskId: null, version: 1 })).toBe(board)
  })
})

describe('withCurrentTask', () => {
  it('places the API copy of a task by its position', () => {
    const board = withCurrentTask(toBoard(response), task(1, 2, 0.5, { version: 3 }))
    expect(ids(board)).toEqual({ backlog: [2], 'in-progress': [1, 3], closed: [4] })
    expect(board.columns[1].tasks[0].version).toBe(3)
  })
})
