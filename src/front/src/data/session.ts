import { useSyncExternalStore } from 'react'
import type { Session } from './types'

// Mock session kept in localStorage so a refresh keeps you signed in.
// Storage can be unavailable (private mode, blocked site data), so every
// access is guarded and the app falls back to in-memory state.

const KEY = 'gsm.session'
const listeners = new Set<() => void>()
let current: Session | null = load()

function load(): Session | null {
  try {
    const raw = localStorage.getItem(KEY)
    return raw ? (JSON.parse(raw) as Session) : null
  } catch {
    return null
  }
}

function set(next: Session | null) {
  current = next
  try {
    if (next) localStorage.setItem(KEY, JSON.stringify(next))
    else localStorage.removeItem(KEY)
  } catch {
    // Keep the in-memory session only.
  }
  listeners.forEach((l) => l())
}

export const saveSession = (s: Session) => set(s)
export const signOut = () => set(null)
export const getSession = () => current

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function useSession(): Session | null {
  return useSyncExternalStore(subscribe, getSession)
}
