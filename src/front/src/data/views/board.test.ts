import { describe, expect, it } from 'vitest'
import { ConflictError, db } from '../mock/store'
import { forceMoveConflict, getBoard, moveTask } from './board'

const task = (id: string) => db.tasks.find((t) => t.id === id)!
const order = async (column: string) =>
  (await getBoard('orbital-nav')).columns.find((c) => c.key === column)!.tasks.map((t) => t.key)

describe('moveTask', () => {
  it('puts the card at the midpoint above beforeTaskId', async () => {
    const before = task('t-44')
    const after = task('t-45')
    await moveTask('orbital-nav', 't-49', { column: 'in-progress', beforeTaskId: 't-45', version: 1 })
    expect(task('t-49').position).toBe((before.position + after.position) / 2)
    expect(await order('in-progress')).toEqual(['PAY-44', 'PAY-49', 'PAY-45', 'PAY-46'])
  })

  it('goes to the top or the bottom of a column', async () => {
    await moveTask('orbital-nav', 't-49', { column: 'in-progress', beforeTaskId: 't-44', version: 1 })
    await moveTask('orbital-nav', 't-51', { column: 'in-progress', beforeTaskId: null, version: 1 })
    expect(await order('in-progress')).toEqual(['PAY-49', 'PAY-44', 'PAY-45', 'PAY-46', 'PAY-51'])
  })

  it('renumbers the column when the gap gets too small', async () => {
    task('t-45').position = task('t-44').position + 1e-9
    await moveTask('orbital-nav', 't-49', { column: 'in-progress', beforeTaskId: 't-45', version: 1 })
    expect(await order('in-progress')).toEqual(['PAY-44', 'PAY-49', 'PAY-45', 'PAY-46'])
  })

  it('bumps the version and logs a moved event', async () => {
    await moveTask('orbital-nav', 't-49', { column: 'in-progress', beforeTaskId: null, version: 1 })
    expect(task('t-49').version).toBe(2)
    expect(db.activityEvents[0]).toMatchObject({ verb: 'moved', taskId: 't-49', detail: 'Moved from Hangar to In orbit' })
  })

  it('lands a card moved into Closed, and requeues it on the way out', async () => {
    await moveTask('orbital-nav', 't-46', { column: 'closed', beforeTaskId: null, version: 1 })
    expect(task('t-46').status).toBe('landed')
    await moveTask('orbital-nav', 't-46', { column: 'testing', beforeTaskId: null, version: 2 })
    expect(task('t-46').status).toBe('queued')
  })

  it('rejects a stale version and changes nothing', async () => {
    await expect(
      moveTask('orbital-nav', 't-49', { column: 'in-progress', beforeTaskId: null, version: 0 }),
    ).rejects.toBeInstanceOf(ConflictError)
    expect(task('t-49')).toMatchObject({ column: 'backlog', version: 1 })
  })

  it('rejects when a conflict is forced', async () => {
    forceMoveConflict()
    await expect(
      moveTask('orbital-nav', 't-49', { column: 'in-progress', beforeTaskId: null, version: 1 }),
    ).rejects.toBeInstanceOf(ConflictError)
    await moveTask('orbital-nav', 't-49', { column: 'in-progress', beforeTaskId: null, version: 1 })
    expect(task('t-49').column).toBe('in-progress')
  })
})
