// Plain names only (AGENT_BRIEF §3). Space display labels live in src/labels.
// Shared by every view: change it only through a small PR into feature/frontend.

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

export const PRIORITIES = ['low', 'medium', 'high', 'critical'] as const
export type Priority = (typeof PRIORITIES)[number]

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
  /** True while an operator has paused it from the console */
  paused: boolean
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
  archived: boolean
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

/** A success criterion (feature) or acceptance criterion (task). */
export interface Criterion {
  id: ID
  text: string
  met: boolean
}

export type FeatureStatus = 'planned' | 'active' | 'landed' | 'archived'

export interface Feature {
  id: ID
  /** Display key, e.g. OBJ-7 */
  key: string
  projectId: ID
  epicId: ID
  name: string
  description: string
  ownerId: ID | null
  sprintId: ID | null
  tags: string[]
  successCriteria: Criterion[]
  /** Default for new tasks; null inherits the project default */
  defaultAutonomy: Autonomy | null
  status: FeatureStatus
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
  description: string
  assignee: Assignee | null
  estimate: number
  priority: Priority
  sprintId: ID | null
  tags: string[]
  acceptanceCriteria: Criterion[]
  /** Overrides the feature default; null inherits */
  autonomyOverride: Autonomy | null
  column: ColumnKey
  position: number
  status: TaskStatus
  blockedReason: string | null
  subtasks: { done: number; total: number }
  prNumber: number | null
  /** Bumped on every change, for move conflicts (ARCHITECTURE.md) */
  version: number
}

export interface Subtask {
  id: ID
  taskId: ID
  title: string
  assignee: Assignee | null
  done: boolean
}

export interface PullRequest {
  number: number
  taskId: ID
  branch: string
  additions: number
  deletions: number
  filesChanged: number
  checks: 'passing' | 'failing' | 'pending'
}

export type TraceKind = 'plan' | 'read' | 'edit' | 'run' | 'result' | 'hail' | 'docking'

/** One step of an agent's work on a task, oldest first. */
export interface TraceStep {
  id: ID
  taskId: ID
  agentId: ID
  at: string
  kind: TraceKind
  text: string
}

export interface Hail {
  id: ID
  taskId: ID
  agentId: ID
  askedAt: string
  question: string
  answer: string | null
  answeredBy: ID | null
  answeredAt: string | null
}

export type ActivityVerb =
  | 'moved'
  | 'created'
  | 'commented'
  | 'hailed'
  | 'answered'
  | 'opened-pr'
  | 'approved'
  | 'requested-changes'
  | 'took-over'
  | 'recalled'
  | 'blocked'

export interface ActivityEvent {
  id: ID
  projectId: ID
  at: string
  actor: Assignee
  verb: ActivityVerb
  taskId: ID | null
  detail: string
  /** True when a human has to act on it */
  needsAction: boolean
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

// ---- Planning flow (AGENT_BRIEF §8) ----

export type DocTopicState = 'covered' | 'asking' | 'not-yet'

export interface DocSection {
  key: string
  title: string
  /** Paragraph text, or a list of items */
  body: string | string[]
  topicState: DocTopicState
}

export interface BriefingMessage {
  id: ID
  from: 'agent' | 'user'
  text: string
  /** Suggested answers the operator can pick */
  options?: string[]
}

export interface DocComment {
  id: ID
  sectionKey: string
  agentId: ID | null
  text: string
}

export type DraftStatus = 'briefing' | 'needs-review' | 'approved' | 'denied' | 'discarded'

export interface FeatureDraft {
  id: ID
  projectId: ID
  name: string
  status: DraftStatus
  version: number
  /** Answers given so far and roughly how many questions there are */
  answered: number
  expectedQuestions: number
  messages: BriefingMessage[]
  sections: DocSection[]
  comments: DocComment[]
  /** The feature created when the doc is approved */
  featureId: ID | null
}

export type ProposalStatus = 'needs-review' | 'approved' | 'approved-edited' | 'sent-back' | 'dropped' | 'revised'

export interface TaskProposal {
  id: ID
  draftId: ID
  order: number
  status: ProposalStatus
  title: string
  description: string
  acceptanceCriteria: string[]
  estimate: number
  priority: Priority
  epicId: ID
  suggestedAssignee: Assignee | null
  dependsOn: ID[]
  /** Doc section this came from, e.g. "Success criteria, item 2" */
  source: string
  agentNote: string
  /** Task created on approval */
  taskId: ID | null
}

export type FeedbackOutcome = 'revise' | 'discard'

export interface Feedback {
  id: ID
  targetType: 'doc' | 'proposal'
  targetId: ID
  /** For a proposal, 'discard' means drop it */
  outcome: FeedbackOutcome
  reasons: string[]
  text: string
  /** Doc section the feedback points at, for doc denials */
  sectionKey: string | null
  at: string
}

// ---- Settings ----

export interface ApprovalRule {
  id: ID
  title: string
  description: string
  enabled: boolean
}

export interface WorkspaceSettings {
  defaultAutonomy: Autonomy
  /** Who may raise an agent to Free flight */
  freeFlightGrantedBy: 'lead' | 'any'
  agentTaskCap: number
  approvalRules: ApprovalRule[]
  provider: { name: string; keyLast4: string | null }
}

export interface Session {
  userId: ID
  email: string
}
