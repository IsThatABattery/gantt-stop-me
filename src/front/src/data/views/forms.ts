// Create and edit forms for features and tasks (feature/frontend-forms).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay, mustFind, newId, NotFoundError, signedInUserId } from '../mock/store'
import { useApiMutation } from '../api/client'
import type { Agent, Assignee, Autonomy, Criterion, Epic, Feature, ID, Priority, Sprint, Subtask, Task, User } from '../types'

/** Choices for the form's selects. */
export interface FormOptions {
  epics: Epic[]
  features: Feature[]
  users: User[]
  agents: Agent[]
  sprints: Sprint[]
}

export function getFormOptions(projectId: ID): Promise<FormOptions> {
  const project = db.projects.find((p) => p.id === projectId)
  if (!project) return Promise.reject(new NotFoundError('Project not found'))
  return delay({
    epics: db.epics.filter((e) => e.projectId === projectId),
    features: db.features.filter((f) => f.projectId === projectId),
    users: db.users.filter((u) => project.memberIds.includes(u.id)),
    agents: db.agents,
    sprints: db.sprints.filter((s) => s.projectId === projectId),
  })
}

export function getFeature(projectId: ID, featureId: ID): Promise<Feature> {
  const f = db.features.find((x) => x.id === featureId && x.projectId === projectId)
  return f ? delay(f) : Promise.reject(new NotFoundError('Feature not found'))
}

/** A feature's tasks, for the list beside the edit form. */
export function listFeatureTasks(projectId: ID, featureId: ID): Promise<Task[]> {
  return delay(db.tasks.filter((t) => t.projectId === projectId && t.featureId === featureId).sort((a, b) => a.key.localeCompare(b.key)))
}

export interface TaskForEdit {
  task: Task
  subtasks: Subtask[]
}

export function getTaskForEdit(projectId: ID, taskId: ID): Promise<TaskForEdit> {
  const task = db.tasks.find((t) => t.id === taskId && t.projectId === projectId)
  if (!task) return Promise.reject(new NotFoundError('Task not found'))
  return delay({ task, subtasks: db.subtasks.filter((s) => s.taskId === taskId) })
}

// ---- Writes ----

/** Fields the create dialog and the edit page share. */
export interface ItemInput {
  name: string
  description: string
  /** Epic for a feature, parent feature for a task */
  parentId: ID
  /** Owner for a feature (always a user), assignee for a task */
  people: Assignee | null
  sprintId: ID | null
  tags: string[]
  /** Task only */
  estimate: number
  priority: Priority
}

export interface FeatureEdit extends ItemInput {
  criteria: Criterion[]
  autonomy: Autonomy | null
}

export type TaskEdit = FeatureEdit

export class ValidationError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ValidationError'
  }
}

function validate(input: ItemInput, kind: 'feature' | 'task') {
  if (!input.name.trim()) throw new ValidationError(kind === 'feature' ? 'Give the objective a name.' : 'Give the payload a title.')
  if (kind === 'task' && (!Number.isFinite(input.estimate) || input.estimate < 0)) {
    throw new ValidationError('Estimate must be 0 or more.')
  }
}

/** Next display key, e.g. PAY-63: one past the highest number in use. */
function nextKey(prefix: string, used: string[]) {
  const nums = used.map((k) => Number(k.slice(prefix.length + 1))).filter(Number.isFinite)
  return `${prefix}-${Math.max(0, ...nums) + 1}`
}

const cleanTags = (tags: string[]) => [...new Set(tags.map((t) => t.trim()).filter(Boolean))]

/** Async so a validation error rejects instead of throwing. */
async function run<T>(fn: () => T): Promise<T> {
  return delay(fn())
}

export function createFeature(projectId: ID, input: ItemInput): Promise<Feature> {
  return run(() => {
    validate(input, 'feature')
    const feature: Feature = {
      id: newId('f'),
      key: nextKey('OBJ', db.features.map((f) => f.key)),
      projectId,
      epicId: input.parentId,
      name: input.name.trim(),
      description: input.description.trim(),
      ownerId: input.people?.kind === 'user' ? input.people.id : null,
      sprintId: input.sprintId,
      tags: cleanTags(input.tags),
      successCriteria: [],
      defaultAutonomy: null,
      status: 'planned',
    }
    db.features.push(feature)
    return feature
  })
}

export function createTask(projectId: ID, input: ItemInput): Promise<Task> {
  return run(() => {
    validate(input, 'task')
    const feature = mustFind(db.features, (f) => f.id === input.parentId && f.projectId === projectId, 'Feature')
    const backlog = db.tasks.filter((t) => t.projectId === projectId && t.column === 'backlog')
    const task: Task = {
      id: newId('t'),
      key: nextKey('PAY', db.tasks.map((t) => t.key)),
      projectId,
      featureId: feature.id,
      epicId: feature.epicId,
      title: input.name.trim(),
      description: input.description.trim(),
      assignee: input.people,
      estimate: input.estimate,
      priority: input.priority,
      sprintId: input.sprintId,
      tags: cleanTags(input.tags),
      acceptanceCriteria: [],
      autonomyOverride: null,
      column: 'backlog',
      position: Math.max(0, ...backlog.map((t) => t.position)) + 1,
      status: 'queued',
      blockedReason: null,
      subtasks: { done: 0, total: 0 },
      prNumber: null,
      version: 1,
    }
    db.tasks.push(task)
    db.activityEvents.push({
      id: newId('ev'),
      projectId,
      at: new Date().toISOString(),
      actor: { kind: 'user', id: signedInUserId },
      verb: 'created',
      taskId: task.id,
      detail: '',
      needsAction: false,
    })
    return task
  })
}

const cleanCriteria = (list: Criterion[]) => list.map((c) => ({ ...c, text: c.text.trim() })).filter((c) => c.text)

export function updateFeature(projectId: ID, featureId: ID, edit: FeatureEdit): Promise<Feature> {
  return run(() => {
    validate(edit, 'feature')
    const f = mustFind(db.features, (x) => x.id === featureId && x.projectId === projectId, 'Feature')
    Object.assign(f, {
      name: edit.name.trim(),
      description: edit.description.trim(),
      epicId: edit.parentId,
      ownerId: edit.people?.kind === 'user' ? edit.people.id : null,
      sprintId: edit.sprintId,
      tags: cleanTags(edit.tags),
      successCriteria: cleanCriteria(edit.criteria),
      defaultAutonomy: edit.autonomy,
    } satisfies Partial<Feature>)
    // A task's phase follows its feature.
    db.tasks.filter((t) => t.featureId === featureId).forEach((t) => (t.epicId = edit.parentId))
    return f
  })
}

export function updateTask(projectId: ID, taskId: ID, edit: TaskEdit): Promise<Task> {
  return run(() => {
    validate(edit, 'task')
    const t = mustFind(db.tasks, (x) => x.id === taskId && x.projectId === projectId, 'Task')
    const feature = mustFind(db.features, (f) => f.id === edit.parentId && f.projectId === projectId, 'Feature')
    Object.assign(t, {
      title: edit.name.trim(),
      description: edit.description.trim(),
      featureId: feature.id,
      epicId: feature.epicId,
      assignee: edit.people,
      sprintId: edit.sprintId,
      tags: cleanTags(edit.tags),
      estimate: edit.estimate,
      priority: edit.priority,
      acceptanceCriteria: cleanCriteria(edit.criteria),
      autonomyOverride: edit.autonomy,
      version: t.version + 1,
    } satisfies Partial<Task>)
    return t
  })
}

export function archiveFeature(projectId: ID, featureId: ID): Promise<Feature> {
  return run(() => {
    const f = mustFind(db.features, (x) => x.id === featureId && x.projectId === projectId, 'Feature')
    f.status = 'archived'
    return f
  })
}

// ---- Hooks ----

export const useFormOptions = (projectId: ID) =>
  useQuery({ queryKey: keys.formOptions(projectId), queryFn: () => getFormOptions(projectId) })

export const useFeature = (projectId: ID, featureId: ID) =>
  useQuery({ queryKey: keys.feature(projectId, featureId), queryFn: () => getFeature(projectId, featureId) })

export const useFeatureTasks = (projectId: ID, featureId: ID) =>
  useQuery({ queryKey: [...keys.feature(projectId, featureId), 'tasks'], queryFn: () => listFeatureTasks(projectId, featureId) })

// A sub-key of keys.task, so it never collides with the drawer's data there,
// and invalidating keys.task refreshes both.
export const useTaskForEdit = (projectId: ID, taskId: ID) =>
  useQuery({ queryKey: [...keys.task(projectId, taskId), 'edit'], queryFn: () => getTaskForEdit(projectId, taskId) })

/** Everything a create or edit can change, for this project. */
const touched = (projectId: ID) => [
  keys.board(projectId),
  keys.formOptions(projectId),
  ['feature', projectId],
  ['task', projectId],
  keys.activity(projectId),
]

export const useCreateFeature = (projectId: ID) =>
  useApiMutation((input: ItemInput) => createFeature(projectId, input), touched(projectId))

export const useCreateTask = (projectId: ID) => useApiMutation((input: ItemInput) => createTask(projectId, input), touched(projectId))

export const useUpdateFeature = (projectId: ID, featureId: ID) =>
  useApiMutation((edit: FeatureEdit) => updateFeature(projectId, featureId, edit), touched(projectId))

export const useUpdateTask = (projectId: ID, taskId: ID) =>
  useApiMutation((edit: TaskEdit) => updateTask(projectId, taskId, edit), touched(projectId))

export const useArchiveFeature = (projectId: ID, featureId: ID) =>
  useApiMutation(() => archiveFeature(projectId, featureId), touched(projectId))
