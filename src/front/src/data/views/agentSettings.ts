// LLM agent settings: planning agents per user, automation agents per project.

import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, useApiMutation } from '../api/client'
import type { AgentSettings, Automation, AutomationInput, PlanningSettings, Provider } from '../api/types'
import { errorCode } from '../errors'
import { keys } from '../keys'
import { listProviders } from './settings'

export const INSTRUCTIONS_MAX = 4000

export const noAgent: AgentSettings = { providerId: null, model: null, instructions: '' }

export const usePlanning = () => useQuery({ queryKey: keys.planning, queryFn: () => api<PlanningSettings>('GET', '/me/planning') })

export const useSavePlanning = () =>
  useApiMutation((s: PlanningSettings) => api<PlanningSettings>('PUT', '/me/planning', s), [keys.planning])

const automationPath = (projectId: string) => `/projects/${projectId}/automation`

/** Rejects with NotFoundError when the server has no runner, or the project is unknown or not the caller's. */
export const useAutomation = (projectId: string) =>
  useQuery({ queryKey: keys.automation(projectId), queryFn: () => api<Automation>('GET', automationPath(projectId)) })

export const useSaveAutomation = (projectId: string) =>
  useApiMutation((input: AutomationInput) => api<Automation>('PUT', automationPath(projectId), input), [keys.automation(projectId)])

export const useTurnOffAutomation = (projectId: string) =>
  useApiMutation(() => api<void>('DELETE', automationPath(projectId)), [keys.automation(projectId)])

export type AgentField = 'provider' | 'model' | 'instructions'
export type AgentErrors = Partial<Record<AgentField, string>>
const fields: AgentField[] = ['provider', 'model', 'instructions']

/** Characters as the server counts them: trimmed, in code points. */
export const instructionsLength = (s: string) => [...s.trim()].length

function instructionsProblem(s: string) {
  if (instructionsLength(s) > INSTRUCTIONS_MAX) return `Use ${INSTRUCTIONS_MAX} characters or fewer.`
  if (s.includes('\0')) return "Instructions can't contain a NUL character."
}

/** Field → message for what's wrong with an agent before saving. `required`: it must have a model. */
export function agentProblems(a: AgentSettings, required = false): AgentErrors {
  const out: AgentErrors = {}
  if (required && a.providerId === null) out.provider = 'Choose a provider.'
  else if (a.providerId !== null && !a.model) out.model = 'Choose a model.'
  const instructions = instructionsProblem(a.instructions)
  if (instructions) out.instructions = instructions
  return out
}

/** The first agent and field with an error, in form order, to focus. */
export function firstError<K extends string>(order: K[], errors: Partial<Record<K, AgentErrors>>) {
  for (const agent of order) {
    const field = fields.find((f) => errors[agent]?.[f])
    if (field) return { agent, field }
  }
  return null
}

const fieldFor: Record<string, AgentField> = {
  invalid_provider: 'provider',
  invalid_model: 'model',
  invalid_instructions: 'instructions',
}

/**
 * Which agent and field a 400 from a save is about. The API doesn't say which
 * agent, so take the first whose value looks wrong against the providers we
 * have; null when none does, and the error belongs to the whole form.
 */
function placeApiError<K extends string>(e: unknown, agents: Record<K, AgentSettings>, providers: Provider[]) {
  const field = fieldFor[errorCode(e) ?? '']
  if (!field) return null
  const agent = (Object.keys(agents) as K[]).find((k) => {
    const a = agents[k]
    const p = providers.find((x) => x.id === a.providerId)
    if (field === 'provider') return a.providerId !== null && !p
    if (field === 'model') return a.providerId !== null && !p?.models.includes(a.model ?? '')
    return !!instructionsProblem(a.instructions)
  })
  return agent ? { agent, field } : null
}

/** placeApiError against a refetched provider list, since a stale one can blame the wrong agent. */
export function usePlaceApiError() {
  const client = useQueryClient()
  return async <K extends string>(e: unknown, agents: Record<K, AgentSettings>) => {
    if (!fieldFor[errorCode(e) ?? '']) return null
    const providers = await client
      .fetchQuery({ queryKey: keys.providers, queryFn: listProviders, staleTime: 0 })
      .catch(() => client.getQueryData<Provider[]>(keys.providers) ?? [])
    return placeApiError(e, agents, providers)
  }
}
