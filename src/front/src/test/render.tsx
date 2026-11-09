import { QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { routes } from '../app/router'
import { createQueryClient } from '../data/queryClient'
import { fake } from './fakeApi'

/**
 * Render the whole app at a path against the fake API, signed in as
 * ada@example.com (with GitHub linked) unless told otherwise.
 */
export function renderApp(path: string, { signedIn = true } = {}) {
  fake.signedInId = signedIn ? 1 : null
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  const client = createQueryClient()
  const utils = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { ...utils, router, client }
}

/** Render one component with the query client and a router around it. */
export function renderWithProviders(ui: ReactElement, { path = '/' } = {}) {
  const router = createMemoryRouter([{ path: '*', element: ui }], { initialEntries: [path] })
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
}
