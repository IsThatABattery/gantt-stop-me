// Plain names only (AGENT_BRIEF §3). Space display labels live in src/labels.

export type ID = string

export type UserRole = 'lead' | 'member'

/** 0 Tethered, 1 Guided, 2 Free flight */
export type Autonomy = 0 | 1 | 2

export type ColumnKey = 'backlog' | 'in-progress' | 'testing' | 'user-check' | 'closed'

export const COLUMN_ORDER: ColumnKey[] = ['backlog', 'in-progress', 'testing', 'user-check', 'closed']

export type TaskStatus =
  | 'queued'
  | 'ready'
  | 'working'
  | 'hail'
  | 'blocked'
  | 'awaiting-approval'
  | 'in-review'
  | 'landed'

export type Priority = 'low' | 'medium' | 'high' | 'critical'

export type AgentState = 'idle' | 'planning' | 'working' | 'hail' | 'docking' | 'blocked'

export interface Workspace {
  id: ID
  name: string
}

export interface User {
  id: ID
  name: string
  initials: string
  email: string
  role: UserRole
}

export interface Agent {
  id: ID
  name: string
  /** What the agent is for, e.g. "Backend builder" */
  role: string
  state: AgentState
  autonomy: Autonomy
  currentTaskId: ID | null
  timeOnTaskMinutes: number
}

export interface Sprint {
  id: ID
  projectId: ID
  number: number
  goal: string
  start: string
  end: string
}

export interface Project {
  id: ID
  workspaceId: ID
  name: string
  description: string
  defaultAutonomy: Autonomy
  activeSprintId: ID | null
  /** Start of the next sprint when none is active */
  nextSprintStart: string | null
  memberIds: ID[]
  /** Accent color of the project icon */
  color: string
  lastActivityAt: string
}

/** Counts the API would compute for the project list */
export interface ProjectStats {
  tasksTotal: number
  tasksLanded: number
  awaitingApproval: number
  blocked: number
  hails: number
  activeAgents: number
}

export interface ProjectSummary extends Project {
  stats: ProjectStats
  activeSprint: Sprint | null
  members: User[]
}

export interface Epic {
  id: ID
  projectId: ID
  name: string
  color: string
}

export interface Feature {
  id: ID
  projectId: ID
  epicId: ID
  name: string
  status: 'planned' | 'active' | 'landed' | 'archived'
}

export type Assignee = { kind: 'user'; id: ID } | { kind: 'agent'; id: ID }

export interface Task {
  id: ID
  /** Display key, e.g. PAY-41 */
  key: string
  projectId: ID
  featureId: ID
  epicId: ID
  title: string
  assignee: Assignee | null
  estimate: number
  priority: Priority
  column: ColumnKey
  position: number
  status: TaskStatus
  blockedReason: string | null
  subtasks: { done: number; total: number }
  prNumber: number | null
}

export interface BoardColumn {
  key: ColumnKey
  tasks: Task[]
}

export interface Board {
  project: Project
  sprint: Sprint | null
  columns: BoardColumn[]
  epics: Epic[]
  features: Feature[]
  users: User[]
  agents: Agent[]
}

export interface Session {
  userId: ID
  email: string
}
