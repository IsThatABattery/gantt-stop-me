// Placeholder data copied from the mockups in docs/ganttstopme-plan/design/mockups.
// Names, IDs, counts and dates are samples, not real data. Views read and write
// this through the mock store (store.ts), never directly.

import type {
  ActivityEvent,
  Agent,
  Criterion,
  DocSection,
  Epic,
  Feature,
  FeatureDraft,
  Hail,
  Project,
  ProjectStats,
  PullRequest,
  Sprint,
  Subtask,
  Task,
  TaskProposal,
  TraceStep,
  User,
  Workspace,
  WorkspaceSettings,
} from '../types'

const MIN = 60 * 1000
const HOUR = 60 * MIN
const DAY = 24 * HOUR
const now = Date.now()
const at = (offsetMs: number) => new Date(now + offsetMs).toISOString()
/** Today at a clock time from the mockups, e.g. "13:20" */
const today = (hhmm: string) => {
  const [h, m] = hhmm.split(':').map(Number)
  const d = new Date(now)
  d.setHours(h, m, 0, 0)
  return d.toISOString()
}
const criteria = (prefix: string, items: [string, boolean][]): Criterion[] =>
  items.map(([text, met], i) => ({ id: `${prefix}-c${i + 1}`, text, met }))

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
  { id: 'a-1', name: 'Astronaut-1', role: 'Backend builder', state: 'docking', autonomy: 1, currentTaskId: 't-44', timeOnTaskMinutes: 52, paused: false },
  { id: 'a-2', name: 'Astronaut-2', role: 'Refactor specialist', state: 'planning', autonomy: 0, currentTaskId: 't-47', timeOnTaskMinutes: 8, paused: false },
  { id: 'a-3', name: 'Astronaut-3', role: 'Test doctor', state: 'hail', autonomy: 1, currentTaskId: 't-45', timeOnTaskMinutes: 21, paused: false },
  { id: 'a-4', name: 'Astronaut-4', role: 'Build and infra', state: 'blocked', autonomy: 2, currentTaskId: 't-43', timeOnTaskMinutes: 70, paused: false },
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
    lastActivityAt: at(-12 * MIN),
    archived: false,
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
    archived: false,
  },
]

export const sprints: Sprint[] = [
  { id: 's-on-14', projectId: 'orbital-nav', number: 14, goal: 'Ship the astronaut approval flow end to end', start: at(-7 * DAY), end: at(3 * DAY + 6 * HOUR) },
  { id: 's-on-15', projectId: 'orbital-nav', number: 15, goal: 'Realtime board updates', start: at(3 * DAY + 6 * HOUR), end: at(17 * DAY) },
  { id: 's-lr-6', projectId: 'lunar-relay', number: 6, goal: 'Stabilise the ground link under load', start: at(-6 * DAY), end: at(8 * DAY + 2 * HOUR) },
]

/** Counts the API would return with the project list. */
export const projectStats: Record<string, ProjectStats> = {
  'orbital-nav': { tasksTotal: 26, tasksLanded: 14, awaitingApproval: 2, blocked: 1, hails: 1, activeAgents: 4 },
  'lunar-relay': { tasksTotal: 18, tasksLanded: 9, awaitingApproval: 1, blocked: 0, hails: 0, activeAgents: 2 },
}

export const epics: Epic[] = [
  { id: 'e-core', projectId: 'orbital-nav', name: 'Core', color: '#3FD5E8' },
  { id: 'e-telemetry', projectId: 'orbital-nav', name: 'Telemetry', color: '#A78BFA' },
  { id: 'e-comms', projectId: 'orbital-nav', name: 'Comms', color: '#FBBF24' },
]

const feature = (f: Partial<Feature> & Pick<Feature, 'id' | 'key' | 'epicId' | 'name'>): Feature => ({
  projectId: 'orbital-nav',
  description: '',
  ownerId: 'u-tv',
  sprintId: 's-on-14',
  tags: [],
  successCriteria: [],
  defaultAutonomy: null,
  status: 'active',
  ...f,
})

export const features: Feature[] = [
  feature({ id: 'f-auth', key: 'OBJ-1', epicId: 'e-core', name: 'Auth service', ownerId: 'u-mk' }),
  feature({
    id: 'f-backlog',
    key: 'OBJ-7',
    epicId: 'e-core',
    name: 'Backlog and priorities',
    description: 'Let the flight director sort and page through the backlog, and set a priority on every payload.',
    tags: ['api', 'backend'],
    successCriteria: criteria('f-backlog', [
      ['Backlog endpoint returns paginated results', true],
      ['Priority can be set from the Hangar', true],
      ['Order persists after reload', false],
    ]),
  }),
  feature({ id: 'f-board', key: 'OBJ-3', epicId: 'e-core', name: 'Board interactions', ownerId: 'u-jl' }),
  feature({ id: 'f-build', key: 'OBJ-4', epicId: 'e-core', name: 'Build tooling', ownerId: 'u-mk' }),
  feature({
    id: 'f-realtime',
    key: 'OBJ-8',
    epicId: 'e-comms',
    name: 'Realtime sync',
    description: 'Boards update live for every operator and astronaut, so nobody works from stale state.',
    sprintId: 's-on-15',
    status: 'planned',
  }),
  feature({ id: 'f-telemetry', key: 'OBJ-5', epicId: 'e-telemetry', name: 'Sprint telemetry' }),
  feature({ id: 'f-theming', key: 'OBJ-6', epicId: 'e-telemetry', name: 'Theming', ownerId: 'u-jl' }),
]

type SubtaskSeed = [title: string, done: boolean, assignee?: Subtask['assignee']]
const a = (id: string) => ({ kind: 'agent' as const, id })
const u = (id: string) => ({ kind: 'user' as const, id })

const subtaskSeeds: Record<string, SubtaskSeed[]> = {
  't-41': [
    ['Add cursor params to the route', true, a('a-1')],
    ['Update the query layer', true, a('a-1')],
    ['Write pagination tests', true, a('a-1')],
    ['Document the response shape flag', false],
  ],
  't-44': [
    ['Pick a generator', true, a('a-1')],
    ['Generate the client', true, a('a-1')],
    ['Replace hand-written calls', true, a('a-1')],
    ['Add a regenerate script', false, a('a-1')],
    ['Remove the old client', false, a('a-1')],
  ],
  't-45': [
    ['Reproduce the failure', true, a('a-3')],
    ['Fix the retry cap mismatch', false, a('a-3')],
    ['Run the test 50 times clean', false, a('a-3')],
  ],
  't-46': [
    ['Capacity query', true, u('u-tv')],
    ['Gauge component', true, u('u-tv')],
    ['Astronaut capacity', false, u('u-tv')],
    ['Empty state', false, u('u-tv')],
  ],
}

export const subtasks: Subtask[] = Object.entries(subtaskSeeds).flatMap(([taskId, seeds]) =>
  seeds.map(([title, done, assignee], i) => ({ id: `${taskId}-m${i + 1}`, taskId, title, done, assignee: assignee ?? null })),
)

type TaskSeed = Pick<Task, 'key' | 'title' | 'epicId' | 'featureId' | 'assignee' | 'estimate' | 'column' | 'status'> &
  Partial<Task>

const seeds: TaskSeed[] = [
  // Hangar · Backlog
  { key: 'PAY-51', title: 'Add loop-closure metrics to telemetry panel', epicId: 'e-telemetry', featureId: 'f-telemetry', assignee: u('u-tv'), estimate: 3, column: 'backlog', status: 'queued' },
  { key: 'PAY-53', title: 'Migration for payload priority field', epicId: 'e-core', featureId: 'f-backlog', assignee: u('u-mk'), estimate: 2, column: 'backlog', status: 'queued' },
  { key: 'PAY-47', title: 'Split auth middleware into its own service', epicId: 'e-core', featureId: 'f-auth', assignee: a('a-2'), estimate: 5, column: 'backlog', status: 'ready', autonomyOverride: 0 },
  { key: 'PAY-49', title: 'Keyboard shortcuts for board navigation', epicId: 'e-core', featureId: 'f-board', assignee: u('u-jl'), estimate: 3, column: 'backlog', status: 'ready' },
  // Approved in the Realtime sync planning flow (PlanDone.dc.html)
  { key: 'PAY-60', title: 'Open a websocket channel per mission', epicId: 'e-comms', featureId: 'f-realtime', assignee: null, estimate: 3, column: 'backlog', status: 'queued', sprintId: 's-on-15' },
  { key: 'PAY-61', title: 'Reconnect with a capped backoff', epicId: 'e-comms', featureId: 'f-realtime', assignee: null, estimate: 2, column: 'backlog', status: 'queued', sprintId: 's-on-15' },
  { key: 'PAY-62', title: 'Broadcast card moves to operators', epicId: 'e-comms', featureId: 'f-realtime', assignee: null, estimate: 3, column: 'backlog', status: 'queued', sprintId: 's-on-15' },
  // In orbit · In Progress
  { key: 'PAY-44', title: 'Generate API client from OpenAPI spec', epicId: 'e-comms', featureId: 'f-realtime', assignee: a('a-1'), estimate: 5, column: 'in-progress', status: 'working' },
  { key: 'PAY-45', title: 'Fix flaky websocket reconnect test', epicId: 'e-comms', featureId: 'f-realtime', assignee: a('a-3'), estimate: 3, column: 'in-progress', status: 'hail' },
  { key: 'PAY-46', title: 'Sprint capacity gauge', epicId: 'e-telemetry', featureId: 'f-telemetry', assignee: u('u-tv'), estimate: 4, column: 'in-progress', status: 'working' },
  // Static fire · Testing
  { key: 'PAY-43', title: 'Upgrade build to Node 22', epicId: 'e-core', featureId: 'f-build', assignee: a('a-4'), estimate: 3, column: 'testing', status: 'blocked', prNumber: 207, blockedReason: 'CI runner image missing' },
  // Docking · User Check
  {
    key: 'PAY-41',
    title: 'Pagination for backlog endpoint',
    epicId: 'e-core',
    featureId: 'f-backlog',
    assignee: a('a-1'),
    estimate: 3,
    column: 'user-check',
    status: 'awaiting-approval',
    prNumber: 212,
    priority: 'high',
    tags: ['api', 'pagination'],
    description: 'Add cursor-based pagination to GET /backlog, default page size 50, and keep the old response shape behind a flag.',
    acceptanceCriteria: criteria('t-41', [
      ['Cursor and page size params accepted', true],
      ['Old response shape kept behind a flag', true],
      ['Tests cover first, middle, and last page', false],
    ]),
  },
  { key: 'PAY-42', title: 'Dark and daylight theme tokens', epicId: 'e-telemetry', featureId: 'f-theming', assignee: u('u-jl'), estimate: 2, column: 'user-check', status: 'in-review', prNumber: 209 },
  // Landed · Closed
  { key: 'PAY-38', title: 'Drag cards between columns', epicId: 'e-core', featureId: 'f-board', assignee: u('u-mk'), estimate: 5, column: 'closed', status: 'landed' },
  { key: 'PAY-39', title: 'Seed demo mission data', epicId: 'e-telemetry', featureId: 'f-board', assignee: a('a-2'), estimate: 2, column: 'closed', status: 'landed' },
]

export const tasks: Task[] = seeds.map((s, i) => {
  const id = 't-' + s.key.slice(4)
  const mine = subtasks.filter((m) => m.taskId === id)
  return {
    description: '',
    priority: 'medium',
    sprintId: 's-on-14',
    tags: [],
    acceptanceCriteria: [],
    autonomyOverride: null,
    blockedReason: null,
    prNumber: null,
    version: 1,
    ...s,
    subtasks: { done: mine.filter((m) => m.done).length, total: mine.length },
    id,
    projectId: 'orbital-nav',
    position: i + 1,
  }
})

export const pullRequests: PullRequest[] = [
  { number: 212, taskId: 't-41', branch: 'backlog-pagination', additions: 184, deletions: 22, filesChanged: 6, checks: 'passing' },
  { number: 209, taskId: 't-42', branch: 'theme-tokens', additions: 96, deletions: 40, filesChanged: 4, checks: 'passing' },
  { number: 207, taskId: 't-43', branch: 'node-22', additions: 31, deletions: 28, filesChanged: 5, checks: 'pending' },
]

export const traceSteps: TraceStep[] = [
  // PayloadDetail.dc.html
  { id: 'tr-41-1', taskId: 't-41', agentId: 'a-1', at: today('13:20'), kind: 'plan', text: 'Add cursor-based pagination to GET /backlog, default page size 50, keep the old response shape behind a flag.' },
  { id: 'tr-41-2', taskId: 't-41', agentId: 'a-1', at: today('13:24'), kind: 'read', text: 'Read routes/backlog.ts and the payload query layer.' },
  { id: 'tr-41-3', taskId: 't-41', agentId: 'a-1', at: today('13:41'), kind: 'edit', text: 'Added cursor params, updated query, wrote 9 tests.' },
  { id: 'tr-41-4', taskId: 't-41', agentId: 'a-1', at: today('14:08'), kind: 'result', text: 'All tests pass locally and in CI.' },
  { id: 'tr-41-5', taskId: 't-41', agentId: 'a-1', at: today('14:12'), kind: 'docking', text: 'Opened PR #212 and requested review from the flight director.' },
  // ProbeConsole.dc.html
  { id: 'tr-45-1', taskId: 't-45', agentId: 'a-3', at: today('13:52'), kind: 'plan', text: 'Reproduce failure, isolate timing.' },
  { id: 'tr-45-2', taskId: 't-45', agentId: 'a-3', at: today('13:55'), kind: 'run', text: 'test:ws ×20 → 4 failures' },
  { id: 'tr-45-3', taskId: 't-45', agentId: 'a-3', at: today('14:01'), kind: 'read', text: 'client/config.ts retryCap = 3' },
  { id: 'tr-45-4', taskId: 't-45', agentId: 'a-3', at: today('14:05'), kind: 'hail', text: 'Retry cap mismatch, asking the flight director.' },
]

export const hails: Hail[] = [
  {
    id: 'h-1',
    taskId: 't-45',
    agentId: 'a-3',
    askedAt: today('14:05'),
    question: 'Should the reconnect retry cap be 3 or 5? The current test assumes 5 but the client config says 3.',
    answer: null,
    answeredBy: null,
    answeredAt: null,
  },
]

export const activityEvents: ActivityEvent[] = [
  { id: 'ev-1', projectId: 'orbital-nav', at: today('14:12'), actor: a('a-1'), verb: 'opened-pr', taskId: 't-41', detail: 'Opened PR #212 and docked for review', needsAction: true },
  { id: 'ev-2', projectId: 'orbital-nav', at: today('14:05'), actor: a('a-3'), verb: 'hailed', taskId: 't-45', detail: 'Asked whether the retry cap should be 3 or 5', needsAction: true },
  { id: 'ev-3', projectId: 'orbital-nav', at: today('13:58'), actor: a('a-4'), verb: 'blocked', taskId: 't-43', detail: 'Comms down: CI runner image missing', needsAction: true },
  { id: 'ev-4', projectId: 'orbital-nav', at: today('13:40'), actor: u('u-jl'), verb: 'opened-pr', taskId: 't-42', detail: 'Opened PR #209', needsAction: false },
  { id: 'ev-5', projectId: 'orbital-nav', at: today('13:31'), actor: u('u-tv'), verb: 'moved', taskId: 't-46', detail: 'Moved from Hangar to In orbit', needsAction: false },
  { id: 'ev-6', projectId: 'orbital-nav', at: today('13:12'), actor: a('a-2'), verb: 'moved', taskId: 't-39', detail: 'Landed after approval', needsAction: false },
  { id: 'ev-7', projectId: 'orbital-nav', at: today('12:47'), actor: u('u-mk'), verb: 'approved', taskId: 't-38', detail: 'Approved and landed', needsAction: false },
  { id: 'ev-8', projectId: 'orbital-nav', at: today('12:20'), actor: u('u-tv'), verb: 'created', taskId: 't-60', detail: 'Added from the Realtime sync plan', needsAction: false },
]

// ---- Planning flow: the Realtime sync objective (PlanBriefing … PlanDone) ----

const reviewedSections: DocSection[] = [
  { key: 'summary', title: 'Summary', topicState: 'covered', body: 'Boards update live for every operator and astronaut, so nobody works from stale state.' },
  { key: 'problem', title: 'Problem', topicState: 'covered', body: 'Operators refresh to see changes and message each other before moving cards. Astronaut moves are invisible until reload, and two simultaneous moves can overwrite each other.' },
  { key: 'goals', title: 'Goals', topicState: 'covered', body: ['Card moves show up for everyone on the mission without a refresh.', 'Simultaneous moves never silently overwrite each other.', 'Astronaut activity is as visible as operator activity.'] },
  { key: 'in-scope', title: 'In scope', topicState: 'covered', body: ['Connection handling and reconnects', 'Broadcasting card moves', 'Showing who else is viewing a card', 'Conflict handling when two people move one card'] },
  { key: 'out-of-scope', title: 'Out of scope', topicState: 'covered', body: ['Editing while fully offline', 'Push notifications on mobile', 'Syncing across missions'] },
  { key: 'success', title: 'Success criteria', topicState: 'covered', body: ['A card move appears for all operators within [TARGET] seconds.', 'A conflicting move shows both positions and lets the second mover choose.', 'The Mission log records every conflict and how it was resolved.'] },
  { key: 'risks', title: 'Risks', topicState: 'covered', body: ['Connection load on busy review days.', 'Conflict prompts could slow operators down.'] },
  { key: 'open-questions', title: 'Open questions', topicState: 'covered', body: ['Should viewers outside the mission see live updates?', 'Who resolves a conflict when an astronaut and an operator collide?'] },
]

const briefingMessages = [
  { id: 'bm-1', from: 'agent' as const, text: 'Tell me about the idea in your own words. What is Realtime sync, and why does the team want it?' },
  { id: 'bm-2', from: 'user' as const, text: 'The board should update live. Right now operators refresh to see changes, and astronaut moves are invisible until reload.' },
  { id: 'bm-3', from: 'agent' as const, text: 'Who is hurt most by stale boards, and what do they do today to work around it?' },
  { id: 'bm-4', from: 'user' as const, text: 'Operators on review days. They message each other before moving a card so they do not collide.' },
  {
    id: 'bm-5',
    from: 'agent' as const,
    text: 'What should happen when two people move the same card at once? Pick a winner, merge the moves, or warn them?',
    options: ['Last write wins', 'Warn and let the mover choose', 'Lock the card while it is moved'],
  },
]

export const featureDrafts: FeatureDraft[] = [
  {
    // Step 1, mid-interview (PlanBriefing.dc.html)
    id: 'realtime-briefing',
    projectId: 'orbital-nav',
    name: 'Realtime sync',
    status: 'briefing',
    version: 0,
    answered: 2,
    expectedQuestions: 8,
    messages: briefingMessages,
    sections: [
      { key: 'problem', title: 'Problem', topicState: 'covered', body: 'Boards go stale. Operators refresh and coordinate by message.' },
      { key: 'users', title: 'Users', topicState: 'covered', body: 'Operators on review days; astronaut moves are invisible until reload.' },
      { key: 'conflicts', title: 'Conflict handling', topicState: 'asking', body: '' },
      { key: 'in-scope', title: 'Scope', topicState: 'not-yet', body: '' },
      { key: 'out-of-scope', title: 'Out of scope', topicState: 'not-yet', body: '' },
      { key: 'success', title: 'Success criteria', topicState: 'not-yet', body: '' },
      { key: 'risks', title: 'Risks', topicState: 'not-yet', body: '' },
    ],
    comments: [],
    featureId: null,
  },
  {
    // Step 2, drafted and waiting for review (PlanDoc.dc.html)
    id: 'realtime-review',
    projectId: 'orbital-nav',
    name: 'Realtime sync',
    status: 'needs-review',
    version: 1,
    answered: 8,
    expectedQuestions: 8,
    messages: briefingMessages,
    sections: reviewedSections,
    comments: [
      { id: 'dc-1', sectionKey: 'open-questions', agentId: null, text: 'I could not tell who should decide a conflict between an astronaut and an operator. Please answer here or deny with feedback.' },
    ],
    featureId: null,
  },
  {
    // Steps 3 and 4, doc approved and payloads proposed (PlanPayloads, PlanDone)
    id: 'realtime',
    projectId: 'orbital-nav',
    name: 'Realtime sync',
    status: 'approved',
    version: 1,
    answered: 8,
    expectedQuestions: 8,
    messages: briefingMessages,
    sections: reviewedSections,
    comments: [],
    featureId: 'f-realtime',
  },
]

const proposal = (p: Partial<TaskProposal> & Pick<TaskProposal, 'order' | 'status' | 'title' | 'estimate'>): TaskProposal => ({
  id: `pr-${p.order}`,
  draftId: 'realtime',
  description: '',
  acceptanceCriteria: [],
  priority: 'medium',
  epicId: 'e-comms',
  suggestedAssignee: null,
  dependsOn: [],
  source: '',
  agentNote: '',
  taskId: null,
  ...p,
})

export const taskProposals: TaskProposal[] = [
  proposal({ order: 1, status: 'approved', title: 'Open a websocket channel per mission', estimate: 3, taskId: 't-60', source: 'In scope, item 1' }),
  proposal({ order: 2, status: 'approved', title: 'Reconnect with a capped backoff', estimate: 2, taskId: 't-61', source: 'In scope, item 1' }),
  proposal({ order: 3, status: 'approved-edited', title: 'Broadcast card moves to operators', estimate: 3, taskId: 't-62', source: 'In scope, item 2' }),
  proposal({
    order: 4,
    status: 'needs-review',
    title: 'Resolve concurrent moves on one card',
    estimate: 5,
    priority: 'high',
    epicId: 'e-core',
    description: 'When two people move the same card, show the second mover both positions and let them choose. Astronaut moves follow the mission autonomy rule.',
    acceptanceCriteria: ['The second mover sees both positions and picks one.', 'The losing move is written to the Mission log.', 'No card ever ends up in two columns.'],
    suggestedAssignee: a('a-2'),
    dependsOn: ['pr-3'],
    source: 'Success criteria, item 2',
    agentNote: 'I split presence into its own payload so this one stays small.',
  }),
  proposal({ order: 5, status: 'needs-review', title: 'Show who else is viewing a card', estimate: 3, source: 'In scope, item 3' }),
  proposal({ order: 6, status: 'needs-review', title: 'Queue and replay moves after a reconnect', estimate: 5, source: 'In scope, item 1' }),
  proposal({ order: 7, status: 'sent-back', title: 'Rate-limit broadcasts per mission', estimate: 2, source: 'Risks, item 1' }),
  proposal({ order: 8, status: 'needs-review', title: 'Sync status in the telemetry bar', estimate: 2, epicId: 'e-telemetry', source: 'Goals, item 1' }),
]

// ---- Settings (Settings.dc.html) ----

export const settings: WorkspaceSettings = {
  defaultAutonomy: 1,
  freeFlightGrantedBy: 'lead',
  agentTaskCap: 4,
  approvalRules: [
    { id: 'rule-dock', title: 'Astronaut output always docks for review', description: 'Nothing an astronaut makes merges without a human, unless it has Free flight.', enabled: true },
    { id: 'rule-two', title: 'Require two approvals on Core phase', description: 'Payloads tagged Core need a second reviewer.', enabled: false },
    { id: 'rule-pause', title: 'Pause astronauts when comms go down', description: 'A blocked astronaut stops instead of retrying.', enabled: true },
    { id: 'rule-hail', title: 'Hails notify the flight director', description: 'Questions from astronauts ping the lead right away.', enabled: true },
  ],
  provider: { name: 'OpenAI', keyLast4: null },
}
