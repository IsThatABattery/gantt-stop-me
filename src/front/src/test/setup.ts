import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { resetDb } from '../data/mock/store'
import { resetFake } from './fakeApi'
import { server } from './server'

// An /api request with no handler fails the test rather than reaching the network.
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))

// Every test starts signed out, on fresh fake API data and fresh mock data.
afterEach(() => {
  cleanup()
  server.resetHandlers()
  resetFake()
  resetDb()
})

afterAll(() => server.close())
