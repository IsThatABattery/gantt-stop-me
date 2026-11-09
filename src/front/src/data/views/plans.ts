// Feature planning: the interviewer, the doc, and the task agent's proposals
// (docs/api/planning.md). The agent works in the background, so a plan is polled
// while `pending` is set.

import { useQuery } from '@tanstack/react-query'
import { api, useApiMutation } from '../api/client'
import type { TaskTest } from '../api/types'
import { keys } from '../keys'

interface PlanMessage {
  id: number
  role: 'agent' | 'user'
  text: string
  options: string[]
  skipped: boolean
}

interface PlanSection {
  key: string
  title: string
  body: string | string[]
  state: 'covered' | 'asking' | 'not_yet'
}

export interface Proposal {
  id: number
  position: number
  status: 'needs_review' | 'approved' | 'approved_edited' | 'sent_back' | 'dropped'
  title: string
  description: string
  acceptanceCriteria: string[]
  tests: TaskTest[]
  dependsOn: number[]
  agentNote: string
  taskId: number | null
}

export interface Plan {
  id: number
  title: string
  description: string
  status: 'briefing' | 'needs_review' | 'approved' | 'done' | 'discarded'
  pending: 'ask' | 'draft' | 'generate' | 'revise' | null
  error: string | null
  doc: PlanSection[]
  messages?: PlanMessage[]
  proposals?: Proposal[]
}

const base = (projectId: string) => `/projects/${projectId}/plans`

export const usePlans = (projectId: string) =>
  useQuery({
    queryKey: keys.plans(projectId),
    queryFn: () => api<{ plans: Plan[] }>('GET', base(projectId)).then((r) => r.plans),
  })

export const usePlan = (projectId: string, planId: string) =>
  useQuery({
    queryKey: keys.plan(projectId, planId),
    queryFn: () => api<Plan>('GET', `${base(projectId)}/${planId}`),
    refetchInterval: (q) => (q.state.data?.pending ? 2000 : false),
  })

export const useStartPlan = (projectId: string) =>
  useApiMutation((input: { title: string; description: string }) => api<Plan>('POST', base(projectId), input), [keys.plans(projectId)])

/** Any POST on a plan or one of its proposals that answers the plan. */
export const usePlanAction = (projectId: string, planId: string) =>
  useApiMutation(
    ({ path, body }: { path: string; body?: unknown }) => api<Plan>('POST', `${base(projectId)}/${planId}/${path}`, body),
    [keys.plan(projectId, planId), keys.plans(projectId), keys.board(projectId)],
  )
