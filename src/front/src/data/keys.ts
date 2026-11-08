import type { ID } from './types'

// Every TanStack Query key in one place, so a write in one view can refresh
// another (e.g. approving in the drawer refreshes the board).
export const keys = {
  workspace: ['workspace'] as const,
  user: (id: ID | undefined) => ['user', id] as const,
  projects: ['projects'] as const,
  project: (projectId: ID) => ['project', projectId] as const,
  board: (projectId: ID) => ['board', projectId] as const,
  task: (projectId: ID, taskId: ID) => ['task', projectId, taskId] as const,
  agents: (projectId: ID) => ['agents', projectId] as const,
  activity: (projectId: ID) => ['activity', projectId] as const,
  formOptions: (projectId: ID) => ['form-options', projectId] as const,
  feature: (projectId: ID, featureId: ID) => ['feature', projectId, featureId] as const,
  draft: (draftId: ID) => ['draft', draftId] as const,
  proposals: (draftId: ID) => ['proposals', draftId] as const,
  settings: ['settings'] as const,
}
