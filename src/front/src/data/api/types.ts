// What the Go API sends, field for field (docs/accounts.md, github.md,
// board.md, providers.md). Ids are numbers. The mock-only screens still use
// the richer types in ../types.ts.

import type { ColumnKey } from '../types'

export interface GitHubLink {
  login: string
  kind: 'oauth' | 'pat'
}

/** GET /api/me */
export interface Me {
  id: number
  email: string
  github: GitHubLink | null
}

/** Signup and login answer the user without `github`. */
export type SignedInUser = Pick<Me, 'id' | 'email'>

export type Permission = 'read' | 'write' | 'admin'

export interface Project {
  id: number
  owner: string
  name: string
  permission: Permission
}

export interface Column {
  id: number
  key: ColumnKey
  name: string
  position: number
  closesIssue: boolean
}

export type RunStatus = 'queued' | 'running' | 'passed' | 'failed' | 'error' | 'cancelled'

export interface Task {
  id: number
  issueNumber: number
  columnId: number
  position: number
  version: number
  title: string
  state: 'open' | 'closed'
  /** The issue on GitHub */
  url: string
  /** The latest test run's status; null when it has none */
  runStatus: RunStatus | null
  /** Why it waits in Testing after a runner error; null while clear */
  needsAttention: string | null
}

export interface Label {
  name: string
  /** Hex without the #, as GitHub gives it */
  color: string
}

export interface TaskTest {
  name: string
  kind: 'script' | 'check'
  command: string
  details: string
  workflow: string | null
}

/** GET /api/projects/{id}/tasks/{taskID} */
export interface TaskDetail extends Task {
  body: string
  labels: Label[]
  tests: TaskTest[]
  testsError: string | null
}

/** GET /api/projects/{id}/board */
export interface BoardResponse {
  project: Project
  columns: Column[]
  tasks: Task[]
}

export interface Provider {
  id: number
  name: string
  baseUrl: string
  /** Last four characters of the key; "" for a key of four or fewer */
  keyLast4: string
  models: string[]
}

/** Create and update also answer the models the endpoint lists. */
export interface ProviderWithModels extends Provider {
  availableModels: string[]
}
