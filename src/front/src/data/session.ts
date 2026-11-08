// Who is signed in. The API keeps the session in an HttpOnly cookie that
// scripts can't read, so GET /api/me is the only way to know (docs/accounts.md).

import { useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api } from './api/client'
import type { Me, SignedInUser } from './api/types'
import { ApiError } from './errors'
import { keys } from './keys'

/** The signed-in user, or null when signed out. */
export async function getMe(): Promise<Me | null> {
  try {
    return await api<Me>('GET', '/me')
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null
    throw e
  }
}

export const useMe = () => useQuery({ queryKey: keys.me, queryFn: getMe, staleTime: 60_000 })

export interface Credentials {
  email: string
  password: string
}

export const signIn = (c: Credentials) => api<SignedInUser>('POST', '/auth/login', c)

export const signUp = (c: Credentials) => api<SignedInUser>('POST', '/auth/signup', c)

/**
 * Forget everything cached for the signed-in user, so the next one starts
 * clean. The user is set to null first, so RequireSession doesn't refetch.
 */
export function forgetSession(client: QueryClient) {
  client.setQueryData(keys.me, null)
  client.removeQueries({ predicate: (q) => q.queryKey[0] !== keys.me[0] })
}

/** End the session, then forget everything cached for this user. */
export function useSignOut() {
  const client = useQueryClient()
  return async () => {
    await api<void>('POST', '/auth/logout').catch(() => undefined)
    forgetSession(client)
  }
}
