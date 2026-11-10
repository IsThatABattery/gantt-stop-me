// Who is signed in. The API keeps the session in an HttpOnly cookie that
// scripts can't read, so GET /api/me is the only way to know (docs/api/accounts.md).

import { hashKey, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from './api/client'
import type { Me, SignedInUser } from './api/types'
import { ApiError } from './errors'
import { keys } from './keys'

/** The signed-in user, or null when signed out. */
async function getMe(): Promise<Me | null> {
  try {
    return await api<Me>('GET', '/me')
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null
    throw e
  }
}

export const useMe = () => useQuery({ queryKey: keys.me, queryFn: getMe, staleTime: 60_000 })

/**
 * useMe for pages built on the user: while the session-ended dialog is up (the user is null), it keeps the last
 * signed-in user, so the page under it keeps its forms and what was typed in them.
 */
export function useSignedInUser() {
  const me = useMe()
  const [last, setLast] = useState(me.data)
  if (me.data && me.data !== last) setLast(me.data)
  return { ...me, data: me.data ?? last }
}

export interface Credentials {
  email: string
  password: string
}

export const signIn = (c: Credentials) => api<SignedInUser>('POST', '/auth/login', c)

export const signUp = (c: Credentials) => api<SignedInUser>('POST', '/auth/signup', c)

/**
 * Continue with GitHub is a full-page navigation, not a fetch: the API redirects to GitHub, and GitHub back to
 * `?next=` (default /), signed in with GitHub linked, or to /login with `?github=` saying why not.
 */
export const GITHUB_SIGN_IN_URL = '/api/auth/github'

/**
 * Forget everything cached for the signed-in user, so the next one starts
 * clean. The user is set to null first, so RequireSession doesn't refetch.
 */
export function forgetSession(client: QueryClient) {
  client.setQueryData(keys.me, null)
  // Whether GitHub sign-in is set up is the server's, not the user's, and /login may be asking it already: removing
  // it would leave that page waiting on a query no one runs.
  client.removeQueries({ predicate: (q) => q.queryKey[0] !== keys.me[0] && q.queryHash !== hashKey(keys.githubOAuth) })
}

/** End the session, then forget everything cached for this user. */
export function useSignOut() {
  const client = useQueryClient()
  return async () => {
    await api<void>('POST', '/auth/logout').catch(() => undefined)
    forgetSession(client)
  }
}
