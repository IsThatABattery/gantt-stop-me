import type { ID } from './types'

// Every TanStack Query key in one place, so a write in one view can refresh
// another (e.g. approving in the drawer refreshes the board).
export const keys = {
  me: ['me'] as const,
  projects: ['projects'] as const,
  assigned: ['assigned'] as const,
  board: (projectId: ID) => ['board', projectId] as const,
  task: (projectId: ID, taskId: ID) => ['task', projectId, taskId] as const,
  providers: ['providers'] as const,
  planning: ['planning'] as const,
  automation: (projectId: ID) => ['automation', projectId] as const,
  plans: (projectId: ID) => ['plans', projectId] as const,
  plan: (projectId: ID, planId: ID) => ['plans', projectId, planId] as const,
  runs: (projectId: ID, taskId: ID) => ['runs', projectId, taskId] as const,
  run: (projectId: ID, runId: ID) => ['run', projectId, runId] as const,
}
