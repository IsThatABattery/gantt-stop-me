import { setupServer } from 'msw/node'
import { handlers } from './fakeApi'

/** Answers every /api request in tests from the fake API. */
export const server = setupServer(...handlers)
