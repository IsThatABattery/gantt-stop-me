// In-memory mock database. Built from the fixtures on page load, so every
// refresh starts from the same sample data. View modules in src/data/views/
// read and write it; components never touch it directly.

import { useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query'
import * as fx from './fixtures'

function fresh() {
  return structuredClone({
    workspace: fx.workspace,
    users: fx.users,
    agents: fx.agents,
    projects: fx.projects,
    projectStats: fx.projectStats,
    sprints: fx.sprints,
    epics: fx.epics,
    features: fx.features,
    tasks: fx.tasks,
    subtasks: fx.subtasks,
    pullRequests: fx.pullRequests,
    traceSteps: fx.traceSteps,
    hails: fx.hails,
    activityEvents: fx.activityEvents,
    featureDrafts: fx.featureDrafts,
    taskProposals: fx.taskProposals,
    feedback: [] as import('../types').Feedback[],
    settings: fx.settings,
  })
}

export type MockDb = ReturnType<typeof fresh>

/** The live mock data. Mutate it only inside api functions. */
export let db: MockDb = fresh()

/** Back to the sample data. Tests call this between cases. */
export function resetDb() {
  db = fresh()
}

export const signedInUserId = fx.signedInUserId

const LATENCY_MS = 150

/** Resolve with a copy after a short fake delay, like a network call. */
export function delay<T>(value: T, ms = LATENCY_MS): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(structuredClone(value)), ms))
}

export class NotFoundError extends Error {
  constructor(what: string) {
    super(`${what} not found`)
    this.name = 'NotFoundError'
  }
}

/** Rejected writes, e.g. a move against a stale version. */
export class ConflictError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ConflictError'
  }
}

/** Find a row or throw NotFoundError. */
export function mustFind<T>(rows: T[], pred: (row: T) => boolean, what: string): T {
  const row = rows.find(pred)
  if (!row) throw new NotFoundError(what)
  return row
}

let nextId = 1000
/** A new unique id with a readable prefix, e.g. newId('t') → "t-1000". */
export const newId = (prefix: string) => `${prefix}-${nextId++}`

/**
 * A TanStack mutation over a mock api function. After it succeeds, every
 * query whose key starts with one of `invalidate` refetches.
 */
export function useMockMutation<TVars, TResult>(fn: (vars: TVars) => Promise<TResult>, invalidate: QueryKey[]) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: () => Promise.all(invalidate.map((queryKey) => client.invalidateQueries({ queryKey }))),
  })
}
