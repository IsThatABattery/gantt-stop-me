// Data every view shares. The current project comes from the API; the rest
// serves only the screens still on mock data (agents, activity, planner,
// objectives, project list), which are hidden until their API exists. See
// docs/implementation/api-gaps.md.

import { useQuery } from '@tanstack/react-query'
import { keys } from './keys'
import { db, delay, mustFind, signedInUserId } from './mock/store'
import type { ID, Project as MockProject, Session, User, Workspace } from './types'
import { getBoard } from './views/board'

export { ApiError, ConflictError, NotFoundError } from './errors'

/** The project, read from its board so the two share one request and one cache entry. */
export const useProject = (projectId: string) =>
  useQuery({ queryKey: keys.board(projectId), queryFn: () => getBoard(projectId), select: (b) => b.project })

// ---- Mock only ----

/** The sample signed-in user the mock screens act as. */
export const useMockSession = (): Session => ({ userId: signedInUserId, email: 'tv@team.dev' })

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

export function getMockProject(projectId: ID): Promise<MockProject> {
  try {
    return delay(mustFind(db.projects, (p) => p.id === projectId, 'Project'))
  } catch (e) {
    return Promise.reject(e)
  }
}

export const useWorkspace = () => useQuery({ queryKey: keys.workspace, queryFn: getWorkspace })

export const useUser = (id: ID | undefined) =>
  useQuery({ queryKey: keys.user(id), queryFn: () => getUser(id!), enabled: !!id })

export const useMockProject = (projectId: ID) =>
  useQuery({ queryKey: keys.project(projectId), queryFn: () => getMockProject(projectId) })
