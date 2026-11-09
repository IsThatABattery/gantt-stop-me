import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { resetFake } from './fakeApi'
import { server } from './server'

// An /api request with no handler fails the test rather than reaching the network.
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))

// Every test starts signed out, on fresh fake API data.
afterEach(() => {
  cleanup()
  server.resetHandlers()
  resetFake()
})

afterAll(() => server.close())
