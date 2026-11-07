// Payload drawer (feature/frontend-task-drawer).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay, NotFoundError } from '../mock/store'
import type { ActivityEvent, Agent, Epic, Feature, Hail, ID, PullRequest, Sprint, Subtask, Task, TraceStep, User } from '../types'

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
  events: ActivityEvent[]
}

export function getTaskDetail(projectId: ID, taskId: ID): Promise<TaskDetail> {
  const task = db.tasks.find((t) => t.id === taskId && t.projectId === projectId)
  if (!task) return Promise.reject(new NotFoundError('Task'))
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
    events: db.activityEvents.filter((e) => e.taskId === taskId),
  })
}

export const useTaskDetail = (projectId: ID, taskId: ID) =>
  useQuery({ queryKey: keys.task(projectId, taskId), queryFn: () => getTaskDetail(projectId, taskId) })
