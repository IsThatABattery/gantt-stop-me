// A task's test runs, approve and merge, and request changes (docs/automation.md).

import { useQuery } from '@tanstack/react-query'
import { api, useApiMutation } from '../api/client'
import type { RunStatus } from '../api/types'
import { keys } from '../keys'

export interface Run {
  id: number
  prNumber: number | null
  status: RunStatus
  reason: string | null
  approvedBy: string | null
  approvedAt: string | null
}

export interface RunResult {
  id: number
  position: number
  text: string
  kind: 'script' | 'llm'
  workflow: string | null
  /** The decision model's pick, by workflow name */
  probabilities: Record<string, number>
  status: 'pending' | 'passed' | 'failed' | 'manual' | 'error' | 'skipped'
  detail: string
  hasScreenshot: boolean
  hasVideo: boolean
}

export interface RunDetail extends Run {
  executions: { id: number; workflow: string; status: RunStatus; logTail: string }[]
  results: RunResult[]
}

const live = (s?: RunStatus) => s === 'queued' || s === 'running'

export const useRuns = (projectId: string, taskId: string) =>
  useQuery({
    queryKey: keys.runs(projectId, taskId),
    queryFn: () => api<{ runs: Run[] }>('GET', `/projects/${projectId}/tasks/${taskId}/runs`).then((r) => r.runs),
    // 404 while the API has no RUNNER_URL: show no runs rather than an error.
    retry: false,
    refetchInterval: (q) => (live(q.state.data?.[0]?.status) ? 3000 : false),
  })

export const useRun = (projectId: string, runId: number | undefined) =>
  useQuery({
    queryKey: keys.run(projectId, String(runId)),
    queryFn: () => api<RunDetail>('GET', `/projects/${projectId}/runs/${runId}`),
    enabled: runId !== undefined,
    refetchInterval: (q) => (live(q.state.data?.status) ? 3000 : false),
  })

const after = (projectId: string, taskId: string) => [keys.board(projectId), keys.task(projectId, taskId), keys.runs(projectId, taskId), ['run', projectId]]

export const useApprove = (projectId: string, taskId: string) =>
  useApiMutation((checked: number[]) => api<Run>('POST', `/projects/${projectId}/tasks/${taskId}/approve`, { checked }), after(projectId, taskId))

export const useRequestChanges = (projectId: string, taskId: string) =>
  useApiMutation(
    (feedback: string) => api('POST', `/projects/${projectId}/tasks/${taskId}/request-changes`, { feedback }),
    after(projectId, taskId),
  )
