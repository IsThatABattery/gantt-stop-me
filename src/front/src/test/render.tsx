import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { routes } from '../app/router'
import { saveSession } from '../data/session'

function client() {
  return new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
}

/** Render the whole app at a path, as a signed-in user unless told otherwise. */
export function renderApp(path: string, { signedIn = true } = {}) {
  if (signedIn) saveSession({ userId: 'u-tv', email: 'tv@team.dev' })
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  const utils = render(
    <QueryClientProvider client={client()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { ...utils, router }
}

/** Render one component with the query client and a router around it. */
export function renderWithProviders(ui: ReactElement, { path = '/' } = {}) {
  const router = createMemoryRouter([{ path: '*', element: ui }], { initialEntries: [path] })
  return render(
    <QueryClientProvider client={client()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
}
