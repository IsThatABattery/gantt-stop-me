// Data every view shares: sign-in, the signed-in user, the workspace and the
// current project. View-specific data lives in src/data/views/<view>.ts.

import { useQuery } from '@tanstack/react-query'
import { keys } from './keys'
import { db, delay, mustFind, signedInUserId } from './mock/store'
import type { ID, Project, Session, User, Workspace } from './types'

export { ConflictError, NotFoundError } from './mock/store'

/** Mock sign-in: accepts anything and signs in as the sample lead. */
export function signIn(email: string): Promise<Session> {
  const user = mustFind(db.users, (u) => u.id === signedInUserId, 'User')
  return delay({ userId: user.id, email: email.trim() || user.email })
}

export function getUser(id: ID): Promise<User> {
  try {
    return delay(mustFind(db.users, (u) => u.id === id, 'User'))
  } catch (e) {
    return Promise.reject(e)
  }
}

export function getWorkspace(): Promise<Workspace> {
  return delay(db.workspace)
}

export function getProject(projectId: ID): Promise<Project> {
  try {
    return delay(mustFind(db.projects, (p) => p.id === projectId, 'Project'))
  } catch (e) {
    return Promise.reject(e)
  }
}

export const useWorkspace = () => useQuery({ queryKey: keys.workspace, queryFn: getWorkspace })

export const useUser = (id: ID | undefined) =>
  useQuery({ queryKey: keys.user(id), queryFn: () => getUser(id!), enabled: !!id })

export const useProject = (projectId: ID) =>
  useQuery({ queryKey: keys.project(projectId), queryFn: () => getProject(projectId) })
