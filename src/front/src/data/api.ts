// Mock data access. Every screen reads data through this module, so it is the
// one place to replace with real API calls later.

import * as db from './mock/fixtures'
import { COLUMN_ORDER, type Board, type ID, type Project, type ProjectSummary, type Session, type User, type Workspace } from './types'

const LATENCY_MS = 150

function delay<T>(value: T): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(structuredClone(value)), LATENCY_MS))
}

export class NotFoundError extends Error {
  constructor(what: string) {
    super(`${what} not found`)
    this.name = 'NotFoundError'
  }
}

/** Mock sign-in: accepts anything and signs in as the sample lead. */
export function signIn(email: string): Promise<Session> {
  const user = db.users.find((u) => u.id === db.signedInUserId)!
  return delay({ userId: user.id, email: email.trim() || user.email })
}

export function getUser(id: ID): Promise<User> {
  const user = db.users.find((u) => u.id === id)
  if (!user) return Promise.reject(new NotFoundError('User'))
  return delay(user)
}

export function getWorkspace(): Promise<Workspace> {
  return delay(db.workspace)
}

export function listProjects(): Promise<ProjectSummary[]> {
  const list = db.projects.map((p) => ({
    ...p,
    stats: db.projectStats[p.id],
    activeSprint: db.sprints.find((s) => s.id === p.activeSprintId) ?? null,
    members: p.memberIds.map((id) => db.users.find((u) => u.id === id)).filter((u) => !!u),
  }))
  list.sort((a, b) => b.lastActivityAt.localeCompare(a.lastActivityAt))
  return delay(list)
}

export function getProject(projectId: ID): Promise<Project> {
  const project = db.projects.find((p) => p.id === projectId)
  if (!project) return Promise.reject(new NotFoundError('Project'))
  return delay(project)
}

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
