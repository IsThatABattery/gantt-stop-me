import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'
import { resetDb } from '../data/mock/store'
import { signOut } from '../data/session'

// Every test starts signed out, on fresh sample data.
afterEach(() => {
  cleanup()
  resetDb()
  signOut()
})
