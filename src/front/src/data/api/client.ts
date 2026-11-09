// The one place the app talks to the Go API. Every route is under /api, on
// the same origin: nginx proxies it in production and Vite does in dev.
// Shapes and error codes are in docs/api/*.md.

import { useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query'
import { ApiError, ConflictError, NotFoundError } from '../errors'

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

interface ErrorBody {
  error?: { code?: string; message?: string }
}

/**
 * Call the API and resolve with its JSON body, or `undefined` for 204.
 * Rejects with ApiError, or NotFoundError / ConflictError for 404 / 409.
 */
export async function api<T>(method: Method, path: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      method,
      credentials: 'same-origin',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'network', "Couldn't reach the server. Check your connection and try again.")
  }

  if (res.status === 204) return undefined as T
  const data: unknown = await res.json().catch(() => null)
  if (res.ok) return data as T

  const err = (data as ErrorBody | null)?.error
  const code = err?.code ?? 'unknown'
  const message = err?.message ?? `Request failed (${res.status}).`
  if (res.status === 404) throw new NotFoundError(message, code, data)
  if (res.status === 409) throw new ConflictError(message, code, data)
  throw new ApiError(res.status, code, message, data)
}

/**
 * A TanStack mutation that refetches every query whose key starts with one of
 * `invalidate` after it succeeds, or after it ends either way with `settled`,
 * for actions whose failure (409, 502) may still have changed the server.
 */
export function useApiMutation<TVars, TResult>(fn: (vars: TVars) => Promise<TResult>, invalidate: QueryKey[], { settled = false } = {}) {
  const client = useQueryClient()
  const refetch = () => Promise.all(invalidate.map((queryKey) => client.invalidateQueries({ queryKey })))
  return useMutation({ mutationFn: fn, ...(settled ? { onSettled: refetch } : { onSuccess: refetch }) })
}
