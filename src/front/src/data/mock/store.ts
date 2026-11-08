// In-memory mock database. Built from the fixtures on page load, so every
// refresh starts from the same sample data. View modules in src/data/views/
// read and write it; components never touch it directly.

import { NotFoundError } from '../errors'
import * as fx from './fixtures'

export { ConflictError, NotFoundError } from '../errors'

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

/** Find a row or throw NotFoundError. */
export function mustFind<T>(rows: T[], pred: (row: T) => boolean, what: string): T {
  const row = rows.find(pred)
  if (!row) throw new NotFoundError(`${what} not found`)
  return row
}

let nextId = 1000
/** A new unique id with a readable prefix, e.g. newId('t') → "t-1000". */
export const newId = (prefix: string) => `${prefix}-${nextId++}`
