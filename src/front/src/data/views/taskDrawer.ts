// Payload drawer (feature/frontend-task-drawer).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay, mustFind, newId, NotFoundError, signedInUserId } from '../mock/store'
import { useApiMutation } from '../api/client'
import type {
  ActivityEvent,
  ActivityVerb,
  Agent,
  ColumnKey,
  Epic,
  Feature,
  Hail,
  ID,
  PullRequest,
  Sprint,
  Subtask,
  Task,
  TraceStep,
  User,
} from '../types'

export interface TaskDetail {
  task: Task
  feature: Feature | null
  epic: Epic | null
  sprint: Sprint | null
  assignee: { kind: 'user'; user: User } | { kind: 'agent'; agent: Agent } | null
  subtasks: Subtask[]
  pullRequest: PullRequest | null
  trace: TraceStep[]
  hails: Hail[]
  /** Newest first */
  events: ActivityEvent[]
  /** Everyone who can appear as an actor in the Flight log */
  users: User[]
  agents: Agent[]
}

export function getTaskDetail(projectId: ID, taskId: ID): Promise<TaskDetail> {
  const task = db.tasks.find((t) => t.id === taskId && t.projectId === projectId)
  if (!task) return Promise.reject(new NotFoundError('Task not found'))
  const a = task.assignee
  const user = a?.kind === 'user' ? db.users.find((u) => u.id === a.id) : undefined
  const agent = a?.kind === 'agent' ? db.agents.find((g) => g.id === a.id) : undefined
  return delay({
    task,
    feature: db.features.find((f) => f.id === task.featureId) ?? null,
    epic: db.epics.find((e) => e.id === task.epicId) ?? null,
    sprint: db.sprints.find((s) => s.id === task.sprintId) ?? null,
    assignee: user ? { kind: 'user', user } : agent ? { kind: 'agent', agent } : null,
    subtasks: db.subtasks.filter((m) => m.taskId === taskId),
    pullRequest: db.pullRequests.find((p) => p.taskId === taskId) ?? null,
    trace: db.traceSteps.filter((s) => s.taskId === taskId),
    hails: db.hails.filter((h) => h.taskId === taskId),
    events: db.activityEvents.filter((e) => e.taskId === taskId).sort((x, y) => y.at.localeCompare(x.at)),
    users: db.users,
    agents: db.agents,
  })
}

export const useTaskDetail = (projectId: ID, taskId: ID) =>
  useQuery({ queryKey: keys.task(projectId, taskId), queryFn: () => getTaskDetail(projectId, taskId) })

/** True when the task is in Mission review and waiting on a human decision. */
export const needsApproval = (t: Pick<Task, 'column' | 'status'>) =>
  t.column === 'user-check' && (t.status === 'awaiting-approval' || t.status === 'in-review')

// ---- Writes ----

/** Run a write inside a promise, so a thrown error becomes a rejection. */
function write<T>(fn: () => T): Promise<T> {
  try {
    return delay(fn())
  } catch (e) {
    return Promise.reject(e)
  }
}

function findTask(projectId: ID, taskId: ID) {
  return mustFind(db.tasks, (t) => t.id === taskId && t.projectId === projectId, 'Task')
}

function logEvent(task: Task, verb: ActivityVerb, detail: string) {
  db.activityEvents.push({
    id: newId('ev'),
    projectId: task.projectId,
    at: new Date().toISOString(),
    actor: { kind: 'user', id: signedInUserId },
    verb,
    taskId: task.id,
    detail,
    needsAction: false,
  })
}

/** Put the task at the end of a column. */
function moveTo(task: Task, column: ColumnKey) {
  if (task.column === column) return
  const last = Math.max(0, ...db.tasks.filter((t) => t.projectId === task.projectId && t.column === column).map((t) => t.position))
  task.column = column
  task.position = last + 1
}

export class ApprovalError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ApprovalError'
  }
}

function mustAwaitApproval(task: Task) {
  if (!needsApproval(task)) throw new ApprovalError(`${task.key} is not waiting for approval.`)
}

/** A decision was made, so the task's log entries stop asking for one. */
function settle(task: Task) {
  for (const e of db.activityEvents) if (e.taskId === task.id) e.needsAction = false
}

interface TaskRef {
  projectId: ID
  taskId: ID
}

/** Approve & land: Mission review → Closed, status landed. */
export function approveTask({ projectId, taskId }: TaskRef): Promise<Task> {
  return write(() => {
    const task = findTask(projectId, taskId)
    mustAwaitApproval(task)
    moveTo(task, 'closed')
    task.status = 'landed'
    task.version++
    settle(task)
    logEvent(task, 'approved', 'Approved and landed')
    return task
  })
}

/** Request changes: back to In Progress with a required note. */
export function requestChanges({ projectId, taskId, note }: TaskRef & { note: string }): Promise<Task> {
  return write(() => {
    const text = note.trim()
    if (!text) throw new ApprovalError('Add a note so the assignee knows what to change.')
    const task = findTask(projectId, taskId)
    mustAwaitApproval(task)
    moveTo(task, 'in-progress')
    task.status = 'working'
    task.version++
    settle(task)
    logEvent(task, 'requested-changes', text)
    return task
  })
}

/** Take over: the signed-in user owns it and keeps working on it. */
export function takeOver({ projectId, taskId }: TaskRef): Promise<Task> {
  return write(() => {
    const task = findTask(projectId, taskId)
    mustAwaitApproval(task)
    task.assignee = { kind: 'user', id: signedInUserId }
    task.status = 'working'
    task.version++
    settle(task)
    logEvent(task, 'took-over', 'Took over from the assignee')
    return task
  })
}

/** Tick a module on or off and keep the task's counts in step. */
export function setSubtaskDone({ projectId, taskId, subtaskId, done }: TaskRef & { subtaskId: ID; done: boolean }): Promise<Subtask> {
  return write(() => {
    const task = findTask(projectId, taskId)
    const sub = mustFind(db.subtasks, (s) => s.id === subtaskId && s.taskId === taskId, 'Subtask')
    sub.done = done
    const mine = db.subtasks.filter((s) => s.taskId === taskId)
    task.subtasks = { done: mine.filter((s) => s.done).length, total: mine.length }
    task.version++
    return sub
  })
}

const refreshAfter = (projectId: ID, taskId: ID) => [keys.board(projectId), keys.task(projectId, taskId), keys.activity(projectId)]

export const useApproveTask = (projectId: ID, taskId: ID) => useApiMutation(approveTask, refreshAfter(projectId, taskId))
export const useRequestChanges = (projectId: ID, taskId: ID) => useApiMutation(requestChanges, refreshAfter(projectId, taskId))
export const useTakeOver = (projectId: ID, taskId: ID) => useApiMutation(takeOver, refreshAfter(projectId, taskId))
export const useSetSubtaskDone = (projectId: ID, taskId: ID) =>
  useApiMutation(setSubtaskDone, [keys.board(projectId), keys.task(projectId, taskId)])
