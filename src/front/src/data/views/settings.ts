// Settings: the user's password, their GitHub link and their LLM providers
// (docs/api/accounts.md, github.md, providers.md).

import { useQuery } from '@tanstack/react-query'
import { api, useApiMutation } from '../api/client'
import type { GitHubLink, Provider, ProviderWithModels } from '../api/types'
import { keys } from '../keys'

// ---- Password ----

export interface PasswordInput {
  current: string
  next: string
  confirm: string
}

/** Field → message for each problem with a password change. Empty when it's fine. */
export function passwordProblems({ current, next, confirm }: PasswordInput): Partial<Record<keyof PasswordInput, string>> {
  const out: Partial<Record<keyof PasswordInput, string>> = {}
  if (!current) out.current = 'Enter your current password.'
  if (next.length < 8) out.next = 'Use 8 or more characters.'
  if (next !== confirm) out.confirm = "The new passwords don't match."
  return out
}

/** Changing it signs out every other session of the user. */
const changePassword = ({ current, next }: PasswordInput) =>
  api<void>('PATCH', '/me/password', { currentPassword: current, newPassword: next })

export const useChangePassword = () => useApiMutation(changePassword, [])

// ---- GitHub ----

/** OAuth is a full-page navigation to GitHub and back, not a fetch. */
export const GITHUB_OAUTH_URL = '/api/github/link'

const linkToken = (token: string) => api<GitHubLink>('POST', '/github/token', { token })

const unlinkGitHub = () => api<void>('DELETE', '/github')

// Linking changes what every project route answers, so everything refetches.
export const useLinkToken = () => useApiMutation(linkToken, [[]])
export const useUnlinkGitHub = () => useApiMutation(unlinkGitHub, [[]])

// ---- LLM providers ----

export const listProviders = () => api<{ providers: Provider[] }>('GET', '/providers').then((r) => r.providers)

export const useProviders = () => useQuery({ queryKey: keys.providers, queryFn: listProviders })

export interface NewProvider {
  name: string
  baseUrl: string
  apiKey: string
  models?: string[]
}

/** Changing `baseUrl` needs `apiKey` again. `{}` only refreshes `availableModels`. */
export interface ProviderEdit {
  baseUrl?: string
  apiKey?: string
  models?: string[]
}

const createProvider = (input: NewProvider) => api<ProviderWithModels>('POST', '/providers', input)

export const updateProvider = (id: number, edit: ProviderEdit) => api<ProviderWithModels>('PATCH', `/providers/${id}`, edit)

const deleteProvider = (id: number) => api<void>('DELETE', `/providers/${id}`)

export const useCreateProvider = () => useApiMutation(createProvider, [keys.providers])

export const useUpdateProvider = (id: number) => useApiMutation((edit: ProviderEdit) => updateProvider(id, edit), [keys.providers])

export const useDeleteProvider = () => useApiMutation(deleteProvider, [keys.providers])
