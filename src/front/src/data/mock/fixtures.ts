// Placeholder data copied from the mockups in ganttstopme-plan/design/mockups.
// Names, IDs, counts and dates are samples, not real data.

import type {
  Agent,
  Epic,
  Feature,
  Project,
  ProjectStats,
  Sprint,
  Task,
  User,
  Workspace,
} from '../types'

const HOUR = 60 * 60 * 1000
const DAY = 24 * HOUR
const now = Date.now()
const at = (offsetMs: number) => new Date(now + offsetMs).toISOString()

export const workspace: Workspace = { id: 'ws-1', name: 'Your org' }

export const users: User[] = [
  { id: 'u-tv', name: 'TV', initials: 'TV', email: 'tv@team.dev', role: 'lead' },
  { id: 'u-mk', name: 'MK', initials: 'MK', email: 'mk@team.dev', role: 'member' },
  { id: 'u-jl', name: 'JL', initials: 'JL', email: 'jl@team.dev', role: 'member' },
  { id: 'u-sr', name: 'SR', initials: 'SR', email: 'sr@team.dev', role: 'member' },
  { id: 'u-ab', name: 'AB', initials: 'AB', email: 'ab@team.dev', role: 'member' },
  { id: 'u-cn', name: 'CN', initials: 'CN', email: 'cn@team.dev', role: 'member' },
]

/** The user every mock sign-in becomes. */
export const signedInUserId = 'u-tv'

export const agents: Agent[] = [
  { id: 'a-1', name: 'Astronaut-1', role: 'Backend builder', state: 'docking', autonomy: 1, currentTaskId: 't-44', timeOnTaskMinutes: 52 },
  { id: 'a-2', name: 'Astronaut-2', role: 'Refactor specialist', state: 'planning', autonomy: 0, currentTaskId: 't-47', timeOnTaskMinutes: 8 },
  { id: 'a-3', name: 'Astronaut-3', role: 'Test doctor', state: 'hail', autonomy: 1, currentTaskId: 't-45', timeOnTaskMinutes: 21 },
  { id: 'a-4', name: 'Astronaut-4', role: 'Build and infra', state: 'blocked', autonomy: 2, currentTaskId: 't-43', timeOnTaskMinutes: 70 },
]

export const projects: Project[] = [
  {
    id: 'orbital-nav',
    workspaceId: 'ws-1',
    name: 'Orbital-Nav',
    description: 'Navigation stack and mission planner.',
    defaultAutonomy: 1,
    activeSprintId: 's-on-14',
    nextSprintStart: null,
    memberIds: ['u-tv', 'u-mk', 'u-jl'],
    color: '#3FD5E8',
    lastActivityAt: at(-12 * 60 * 1000),
  },
  {
    id: 'lunar-relay',
    workspaceId: 'ws-1',
    name: 'Lunar Relay',
    description: 'Telemetry relay service and ground link.',
    defaultAutonomy: 1,
    activeSprintId: 's-lr-6',
    nextSprintStart: null,
    memberIds: ['u-sr', 'u-tv'],
    color: '#A78BFA',
    lastActivityAt: at(-1 * HOUR),
  },
  {
    id: 'deep-field',
    workspaceId: 'ws-1',
    name: 'Deep Field',
    description: 'Data pipeline for long-running analysis.',
    defaultAutonomy: 0,
    activeSprintId: null,
    nextSprintStart: at(9 * DAY),
    memberIds: ['u-ab', 'u-cn', 'u-tv', 'u-mk'],
    color: '#4ADE80',
    lastActivityAt: at(-2 * DAY),
  },
  {
    id: 'ground-station',
    workspaceId: 'ws-1',
    name: 'Ground Station',
    description: 'Admin console and operator tooling.',
    defaultAutonomy: 1,
    activeSprintId: 's-gs-3',
    nextSprintStart: null,
    memberIds: ['u-jl', 'u-sr'],
    color: '#FBBF24',
    lastActivityAt: at(-3 * HOUR),
  },
]

export const sprints: Sprint[] = [
  { id: 's-on-14', projectId: 'orbital-nav', number: 14, goal: 'Ship the astronaut approval flow end to end', start: at(-7 * DAY), end: at(3 * DAY + 6 * HOUR) },
  { id: 's-lr-6', projectId: 'lunar-relay', number: 6, goal: 'Stabilise the ground link under load', start: at(-6 * DAY), end: at(8 * DAY + 2 * HOUR) },
  { id: 's-gs-3', projectId: 'ground-station', number: 3, goal: 'Operator tooling for roster changes', start: at(-12 * DAY), end: at(1 * DAY + 20 * HOUR) },
]

/** Counts the API would return with the project list. */
export const projectStats: Record<string, ProjectStats> = {
  'orbital-nav': { tasksTotal: 26, tasksLanded: 14, awaitingApproval: 2, blocked: 1, hails: 1, activeAgents: 4 },
  'lunar-relay': { tasksTotal: 18, tasksLanded: 9, awaitingApproval: 1, blocked: 0, hails: 0, activeAgents: 2 },
  'deep-field': { tasksTotal: 31, tasksLanded: 31, awaitingApproval: 0, blocked: 0, hails: 0, activeAgents: 0 },
  'ground-station': { tasksTotal: 12, tasksLanded: 5, awaitingApproval: 0, blocked: 0, hails: 0, activeAgents: 1 },
}

export const epics: Epic[] = [
  { id: 'e-core', projectId: 'orbital-nav', name: 'Core', color: '#3FD5E8' },
  { id: 'e-telemetry', projectId: 'orbital-nav', name: 'Telemetry', color: '#A78BFA' },
  { id: 'e-comms', projectId: 'orbital-nav', name: 'Comms', color: '#FBBF24' },
]

export const features: Feature[] = [
  { id: 'f-auth', projectId: 'orbital-nav', epicId: 'e-core', name: 'Auth service', status: 'active' },
  { id: 'f-backlog', projectId: 'orbital-nav', epicId: 'e-core', name: 'Backlog and priorities', status: 'active' },
  { id: 'f-board', projectId: 'orbital-nav', epicId: 'e-core', name: 'Board interactions', status: 'active' },
  { id: 'f-build', projectId: 'orbital-nav', epicId: 'e-core', name: 'Build tooling', status: 'active' },
  { id: 'f-realtime', projectId: 'orbital-nav', epicId: 'e-comms', name: 'Realtime sync', status: 'planned' },
  { id: 'f-telemetry', projectId: 'orbital-nav', epicId: 'e-telemetry', name: 'Sprint telemetry', status: 'active' },
  { id: 'f-theming', projectId: 'orbital-nav', epicId: 'e-telemetry', name: 'Theming', status: 'active' },
]

type TaskSeed = Omit<Task, 'id' | 'projectId' | 'position' | 'priority' | 'blockedReason' | 'subtasks' | 'prNumber'> &
  Partial<Pick<Task, 'priority' | 'blockedReason' | 'subtasks' | 'prNumber'>>

const seeds: TaskSeed[] = [
  // Hangar · Backlog
  { key: 'PAY-51', title: 'Add loop-closure metrics to telemetry panel', epicId: 'e-telemetry', featureId: 'f-telemetry', assignee: { kind: 'user', id: 'u-tv' }, estimate: 3, column: 'backlog', status: 'queued' },
  { key: 'PAY-53', title: 'Migration for payload priority field', epicId: 'e-core', featureId: 'f-backlog', assignee: { kind: 'user', id: 'u-mk' }, estimate: 2, column: 'backlog', status: 'queued' },
  { key: 'PAY-47', title: 'Split auth middleware into its own service', epicId: 'e-core', featureId: 'f-auth', assignee: { kind: 'agent', id: 'a-2' }, estimate: 5, column: 'backlog', status: 'ready' },
  { key: 'PAY-49', title: 'Keyboard shortcuts for board navigation', epicId: 'e-core', featureId: 'f-board', assignee: { kind: 'user', id: 'u-jl' }, estimate: 3, column: 'backlog', status: 'ready' },
  // In orbit · In Progress
  { key: 'PAY-44', title: 'Generate API client from OpenAPI spec', epicId: 'e-comms', featureId: 'f-realtime', assignee: { kind: 'agent', id: 'a-1' }, estimate: 5, column: 'in-progress', status: 'working', subtasks: { done: 3, total: 5 } },
  { key: 'PAY-45', title: 'Fix flaky websocket reconnect test', epicId: 'e-comms', featureId: 'f-realtime', assignee: { kind: 'agent', id: 'a-3' }, estimate: 3, column: 'in-progress', status: 'hail', subtasks: { done: 1, total: 3 } },
  { key: 'PAY-46', title: 'Sprint capacity gauge', epicId: 'e-telemetry', featureId: 'f-telemetry', assignee: { kind: 'user', id: 'u-tv' }, estimate: 4, column: 'in-progress', status: 'working', subtasks: { done: 2, total: 4 } },
  // Static fire · Testing
  { key: 'PAY-43', title: 'Upgrade build to Node 22', epicId: 'e-core', featureId: 'f-build', assignee: { kind: 'agent', id: 'a-4' }, estimate: 3, column: 'testing', status: 'blocked', prNumber: 207, blockedReason: 'CI runner image missing' },
  // Docking · User Check
  { key: 'PAY-41', title: 'Pagination for backlog endpoint', epicId: 'e-core', featureId: 'f-backlog', assignee: { kind: 'agent', id: 'a-1' }, estimate: 3, column: 'user-check', status: 'awaiting-approval', prNumber: 212 },
  { key: 'PAY-42', title: 'Dark and daylight theme tokens', epicId: 'e-telemetry', featureId: 'f-theming', assignee: { kind: 'user', id: 'u-jl' }, estimate: 2, column: 'user-check', status: 'in-review', prNumber: 209 },
  // Landed · Closed
  { key: 'PAY-38', title: 'Drag cards between columns', epicId: 'e-core', featureId: 'f-board', assignee: { kind: 'user', id: 'u-mk' }, estimate: 5, column: 'closed', status: 'landed' },
  { key: 'PAY-39', title: 'Seed demo mission data', epicId: 'e-telemetry', featureId: 'f-board', assignee: { kind: 'agent', id: 'a-2' }, estimate: 2, column: 'closed', status: 'landed' },
]

export const tasks: Task[] = seeds.map((s, i) => ({
  priority: 'medium',
  blockedReason: null,
  subtasks: { done: 0, total: 0 },
  prNumber: null,
  ...s,
  id: 't-' + s.key.slice(4),
  projectId: 'orbital-nav',
  position: i + 1,
}))
