// One task: its detail for the drawer and edit page, and creating and editing
// it. A task is a GitHub issue; title and body write through to it
// (docs/board.md).

import { useQuery } from '@tanstack/react-query'
import { api, useApiMutation } from '../api/client'
import type { GitHubUser, Task, TaskDetail } from '../api/types'
import { keys } from '../keys'

export const getTask = (projectId: string, taskId: string) =>
  api<TaskDetail>('GET', `/projects/${projectId}/tasks/${taskId}`)

export const useTask = (projectId: string, taskId: string) =>
  useQuery({ queryKey: keys.task(projectId, taskId), queryFn: () => getTask(projectId, taskId) })

export interface TaskInput {
  title: string
  body: string
}

export const createTask = (projectId: string, input: TaskInput) => api<Task>('POST', `/projects/${projectId}/tasks`, input)

/** Send only what changed; the API wants at least one of the two. */
export const editTask = (projectId: string, taskId: string, edit: Partial<TaskInput>) =>
  api<Task>('PATCH', `/projects/${projectId}/tasks/${taskId}`, edit)

export const useCreateTask = (projectId: string) =>
  useApiMutation((input: TaskInput) => createTask(projectId, input), [keys.board(projectId)])

export const useEditTask = (projectId: string, taskId: string) =>
  useApiMutation((edit: Partial<TaskInput>) => editTask(projectId, taskId, edit), [keys.board(projectId), keys.task(projectId, taskId)])

/** Who the project's tasks can be assigned to (GET /api/projects/{id}/assignees). */
export const useAssignable = (projectId: string) =>
  useQuery({
    queryKey: ['assignable', projectId],
    queryFn: () => api<{ assignees: GitHubUser[] }>('GET', `/projects/${projectId}/assignees`).then((r) => r.assignees),
    staleTime: 5 * 60_000,
  })

/** Replaces the task's assignees on its GitHub issue. */
export const useAssign = (projectId: string, taskId: string) =>
  useApiMutation(
    (assignees: string[]) => api<Task>('PATCH', `/projects/${projectId}/tasks/${taskId}`, { assignees }),
    [keys.board(projectId), keys.task(projectId, taskId)],
  )

/** Longest title the API takes, after trimming. */
export const MAX_TITLE = 256

/** A problem the API would refuse, caught before sending. */
export function titleProblem(title: string) {
  const t = title.trim()
  if (!t) return 'Give it a title.'
  if (t.length > MAX_TITLE) return `Keep the title to ${MAX_TITLE} characters.`
  return null
}
