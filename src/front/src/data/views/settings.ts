// Settings (feature/frontend-settings).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay, mustFind } from '../mock/store'
import { useApiMutation } from '../api/client'
import type { Autonomy, ID, User, WorkspaceSettings } from '../types'

export function getSettings(): Promise<WorkspaceSettings> {
  return delay(db.settings)
}

export const useSettings = () => useQuery({ queryKey: keys.settings, queryFn: getSettings })

export class SettingsError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'SettingsError'
  }
}

/** Run a write inside a promise, so a thrown error becomes a rejection. */
function write<T>(fn: () => T): Promise<T> {
  try {
    return delay(fn())
  } catch (e) {
    return Promise.reject(e)
  }
}

export interface SettingsInput {
  defaultAutonomy: Autonomy
  freeFlightGrantedBy: WorkspaceSettings['freeFlightGrantedBy']
  agentTaskCap: number
  /** Rule id → on/off */
  rules: Record<ID, boolean>
  providerName: string
  /** A new key to store. Empty keeps the current one. Only its last 4 characters are kept. */
  apiKey: string
}

export function saveSettings(input: SettingsInput): Promise<WorkspaceSettings> {
  return write(() => {
    if (!Number.isInteger(input.agentTaskCap) || input.agentTaskCap < 1) {
      throw new SettingsError('Concurrent astronaut payloads must be a whole number, 1 or more.')
    }
    const s = db.settings
    s.defaultAutonomy = input.defaultAutonomy
    s.freeFlightGrantedBy = input.freeFlightGrantedBy
    s.agentTaskCap = input.agentTaskCap
    s.approvalRules = s.approvalRules.map((r) => ({ ...r, enabled: input.rules[r.id] ?? r.enabled }))
    const key = input.apiKey.trim()
    // ARCHITECTURE.md "Provider settings": the full key is never kept or shown.
    s.provider = { name: input.providerName, keyLast4: key ? key.slice(-4) : s.provider.keyLast4 }
    return s
  })
}

export const useSaveSettings = () => useApiMutation(saveSettings, [keys.settings])

export interface ProfileInput {
  userId: ID
  name: string
  email: string
}

export function saveProfile({ userId, name, email }: ProfileInput): Promise<User> {
  return write(() => {
    const n = name.trim()
    const e = email.trim()
    if (!n) throw new SettingsError('Enter your name.')
    if (!/^\S+@\S+\.\S+$/.test(e)) throw new SettingsError('Enter a valid email address.')
    const user = mustFind(db.users, (u) => u.id === userId, 'User')
    user.name = n
    user.email = e
    return user
  })
}

export const useSaveProfile = (userId: ID) => useApiMutation(saveProfile, [keys.user(userId)])

export interface PasswordInput {
  current: string
  next: string
  confirm: string
}

/** Field → message for each problem with a password change. Empty when it's fine. */
export function passwordProblems({ current, next, confirm }: PasswordInput): Partial<Record<keyof PasswordInput, string>> {
  const out: Partial<Record<keyof PasswordInput, string>> = {}
  if (!current) out.current = 'Enter your current password.'
  if (next.length < 8) out.next = 'Use 8 or more characters.'
  if (next !== confirm) out.confirm = "The new passwords don't match."
  return out
}

/** Mock only: checks the shape, never a real password. */
export function changePassword(input: PasswordInput): Promise<void> {
  return write(() => {
    const problems = Object.values(passwordProblems(input))
    if (problems.length) throw new SettingsError(problems[0])
  })
}

export const useChangePassword = () => useApiMutation(changePassword, [])
