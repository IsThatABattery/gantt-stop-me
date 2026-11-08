// A project's workflows file, .gantt/workflows.yml on the default branch (docs/automation.md).

import { useQuery } from '@tanstack/react-query'
import { api, useApiMutation } from '../api/client'

export interface WorkflowsFile {
  /** "" without a file */
  content: string
  /** The blob sha to save over; null without a file */
  sha: string | null
  /** What runs use: the file's, or the one default workflow */
  workflows: { name: string; description: string }[]
  /** Why the file doesn't parse */
  error: string | null
}

const key = (projectId: string) => ['workflows', projectId]

export const useWorkflows = (projectId: string) =>
  useQuery({ queryKey: key(projectId), queryFn: () => api<WorkflowsFile>('GET', `/projects/${projectId}/workflows`) })

export const useSaveWorkflows = (projectId: string) =>
  useApiMutation(
    (input: { content: string; sha: string | null }) => api<WorkflowsFile>('PUT', `/projects/${projectId}/workflows`, input),
    [key(projectId)],
  )
