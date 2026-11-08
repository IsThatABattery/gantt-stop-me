import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { ApiError } from './errors'
import { keys } from './keys'

/**
 * The app's query client. Any 401 means the session is gone (expired, or
 * signed out in another tab), so the signed-in user is cleared and
 * RequireSession sends the user to the login page.
 */
export function createQueryClient() {
  const onError = (error: Error) => {
    if (error instanceof ApiError && error.status === 401) client.setQueryData(keys.me, null)
  }
  const client: QueryClient = new QueryClient({
    queryCache: new QueryCache({ onError }),
    mutationCache: new MutationCache({ onError }),
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false }, mutations: { retry: false } },
  })
  return client
}
