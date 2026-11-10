// Settings: the user's password, their GitHub link and their LLM providers
// (docs/api/accounts.md, github.md, providers.md).

import { useQuery } from '@tanstack/react-query'
import { api, useApiMutation } from '../api/client'
import type { GitHubLink, GitHubOAuth, Provider, ProviderWithModels } from '../api/types'
import { keys } from '../keys'
import { GITHUB_SIGN_IN_URL } from '../session'

// ---- Password ----

export interface PasswordInput {
  current: string
  next: string
  confirm: string
}

/**
 * Field → message for each problem with a password change, or with setting the first one (`setting`), which has no
 * current password. Empty when it's fine.
 */
export function passwordProblems({ current, next, confirm }: PasswordInput, setting = false): Partial<Record<keyof PasswordInput, string>> {
  const out: Partial<Record<keyof PasswordInput, string>> = {}
  if (!setting && !current) out.current = 'Enter your current password.'
  if (next.length < 8) out.next = 'Use 8 or more characters.'
  if (next !== confirm) out.confirm = "The new passwords don't match."
  return out
}

/**
 * Changing or setting it signs out every other session of the user. An account made by signing in with GitHub has
 * none: setting it sends only the new one, from a session made in the last 10 minutes, else 403 reauth_required.
 */
const changePassword = ({ current, next }: PasswordInput, setting: boolean) =>
  api<void>('PATCH', '/me/password', setting ? { newPassword: next } : { currentPassword: current, newPassword: next })

/** Signing in with GitHub again makes the fresh session setting a first password needs, then comes back to Settings. */
export const GITHUB_REAUTH_URL = `${GITHUB_SIGN_IN_URL}?next=/settings`

// The user refetches: setting one turns hasPassword on. It refetches when refused too: 403 wrong_password to a set
// means another session set one first, and the refetch turns the form into the change form, the message on Current.
export const useChangePassword = (setting: boolean) =>
  useApiMutation((input: PasswordInput) => changePassword(input, setting), [keys.me], { settled: true })

// ---- GitHub ----

/**
 * OAuth is a full-page navigation to GitHub and back, not a fetch. A user with a password whose session is over 10
 * minutes old, and not confirmed since, is sent back first with ?github=confirm.
 */
export const GITHUB_OAUTH_URL = '/api/github/link'

/**
 * Confirms the password on this session, so linking with OAuth from it goes through for the next 10 minutes. 403
 * wrong_password; 409 password_required for an account with none.
 */
const confirmPassword = (password: string) => api<void>('POST', '/auth/confirm', { password })

export const useConfirmPassword = () => useApiMutation(confirmPassword, [])

/**
 * Whether the server's GitHub OAuth app is set up, for linking here and Continue with GitHub on sign in and sign up.
 * It needs no session, so asking never ends one. It changes when the server restarts with it set or unset, so it is
 * asked again when the tab regains focus: a server just set up shows Link with GitHub without a reload.
 */
export const useGitHubOAuth = () =>
  useQuery({ queryKey: keys.githubOAuth, queryFn: () => api<GitHubOAuth>('GET', '/github/oauth'), refetchOnWindowFocus: true })

const linkToken = (token: string) => api<GitHubLink>('POST', '/github/token', { token })

/** Refused with 409 password_required while the account has no password: GitHub is then its only way in. */
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
