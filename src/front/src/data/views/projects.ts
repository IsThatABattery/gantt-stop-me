// Mission select (feature/frontend-projects).

import { useQuery } from '@tanstack/react-query'
import { useCallback, useSyncExternalStore } from 'react'
import { keys } from '../keys'
import { db, delay } from '../mock/store'
import { useApiMutation } from '../api/client'
import type { ID, Project, ProjectSummary } from '../types'

export function listProjects(): Promise<ProjectSummary[]> {
  const list = db.projects.map((p) => ({
    ...p,
    stats: db.projectStats[p.id],
    activeSprint: db.sprints.find((s) => s.id === p.activeSprintId) ?? null,
    members: p.memberIds.map((id) => db.users.find((u) => u.id === id)).filter((u) => !!u),
  }))
  list.sort((a, b) => b.lastActivityAt.localeCompare(a.lastActivityAt))
  return delay(list)
}

export const useProjects = () => useQuery({ queryKey: keys.projects, queryFn: listProjects })

// ---- New mission ----

export interface NewProjectInput {
  name: string
  description: string
  /** Signed-in user, who becomes the only member */
  userId: ID
}

/** Accent colors for new mission icons, in turn. */
const COLORS = ['#3FD5E8', '#A78BFA', '#4ADE80', '#FBBF24']

function slug(name: string) {
  const base =
    name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '') || 'mission'
  let id = base
  for (let n = 2; db.projects.some((p) => p.id === id); n++) id = `${base}-${n}`
  return id
}

export async function createProject({ name, description, userId }: NewProjectInput): Promise<Project> {
  if (!name.trim()) throw new Error('Give the mission a name.')
  const project: Project = {
    id: slug(name),
    workspaceId: db.workspace.id,
    name: name.trim(),
    description: description.trim(),
    defaultAutonomy: db.settings.defaultAutonomy,
    activeSprintId: null,
    nextSprintStart: null,
    memberIds: [userId],
    color: COLORS[db.projects.length % COLORS.length],
    lastActivityAt: new Date().toISOString(),
    archived: false,
  }
  db.projects.push(project)
  db.projectStats[project.id] = { tasksTotal: 0, tasksLanded: 0, awaitingApproval: 0, blocked: 0, hails: 0, activeAgents: 0 }
  return delay(project)
}

export const useCreateProject = () => useApiMutation(createProject, [keys.projects])

// ---- Favorites ----
// Not in the data model yet, so kept per user in localStorage. Storage can be
// unavailable (private mode, blocked site data): every access is guarded and
// falls back to memory, as in data/session.ts.

const favKey = (userId: ID) => `gsm.favorites.${userId}`
const memory = new Map<string, ID[]>()
const listeners = new Set<() => void>()
const EMPTY: ID[] = []

function readFavorites(userId: ID): ID[] {
  const key = favKey(userId)
  if (!memory.has(key)) {
    let stored: ID[] = []
    try {
      const raw = localStorage.getItem(key)
      const parsed: unknown = raw ? JSON.parse(raw) : []
      if (Array.isArray(parsed)) stored = parsed.filter((x): x is ID => typeof x === 'string')
    } catch {
      // Start with none.
    }
    memory.set(key, stored)
  }
  return memory.get(key) ?? EMPTY
}

export function toggleFavorite(userId: ID, projectId: ID) {
  const current = readFavorites(userId)
  const next = current.includes(projectId) ? current.filter((id) => id !== projectId) : [...current, projectId]
  memory.set(favKey(userId), next)
  try {
    localStorage.setItem(favKey(userId), JSON.stringify(next))
  } catch {
    // Keep the in-memory copy only.
  }
  listeners.forEach((l) => l())
}

/** Forget cached favorites, so the next read goes back to storage. Tests call it. */
export function clearFavoritesCache() {
  memory.clear()
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/** The user's favorite project ids, and a toggle. */
export function useFavorites(userId: ID | undefined) {
  const favorites = useSyncExternalStore(subscribe, () => (userId ? readFavorites(userId) : EMPTY))
  const toggle = useCallback((projectId: ID) => userId && toggleFavorite(userId, projectId), [userId])
  return { favorites, toggle }
}
