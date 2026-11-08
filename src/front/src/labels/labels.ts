import type { RunStatus } from '../data/api/types'
import type {
  ActivityVerb,
  AgentState,
  Autonomy,
  ColumnKey,
  DocTopicState,
  DraftStatus,
  FeatureStatus,
  Priority,
  ProposalStatus,
  TaskStatus,
  TraceKind,
  UserRole,
} from '../data/types'

// Display labels for the Space theme. Code, data and URLs use plain names;
// every themed word the user sees comes from here, so another theme can be
// added later by swapping this object. Shared by every view: change it only
// through a small PR into feature/frontend.
export const labels = {
  appName: 'GanttStopMe',

  workspace: 'Space agency',
  project: 'Mission',
  projects: 'Missions',
  epic: 'Mission phase',
  feature: 'Objective',
  features: 'Objectives',
  task: 'Payload',
  tasks: 'Payloads',
  subtask: 'Module',
  backlog: 'Hangar',
  sprint: 'Launch window',
  sprints: 'Launch windows',
  user: 'Operator',
  users: 'Operators',
  agent: 'Astronaut',
  agents: 'Astronauts',
  board: 'Mission control',
  activity: 'Mission log',
  retro: 'Debrief',
  settings: 'Settings',
  automation: 'Automation',
  plans: 'Plan a feature',

  allProjects: 'All missions',
  addProject: 'Add a mission',
  newProject: 'New mission',
  testsRunning: 'Static fires running',
  linkGitHub: 'Link GitHub',
  newProjectHint: 'Start from scratch or import a backlog',
  selectProject: 'Select a mission',
  searchProjects: 'Search missions',
  flightDeck: 'Flight deck',
  flightDeckAccess: 'Flight deck access',
  newUser: 'New operator?',
  requestAccess: 'Request access',
  needsYou: 'Needs you',
  reviewNow: 'Review now',
  tasksLanded: 'Payloads landed',
  newFeature: 'New objective',
  newTask: 'New payload',
  manageFeatures: 'Manage objectives',
  allFeatures: 'All objectives',
  tMinus: 'T-minus',
  awaitingApproval: 'Awaiting approval',
  blocked: 'Comms down',
  agentsActive: 'Astronauts active',
  hail: 'Hail',
  hails: 'Hails',
  sprintName: (n: number) => `Launch window ${n}`,
  inSprint: (n: number) => `In launch window ${n}`,
  nextSprint: (date: string) => `Next: ${date}`,
  betweenSprints: 'Between launch windows',
  lead: 'Flight director',
  operatorsOnly: 'Operators only',
  agentsOnly: 'Astronauts only',
  needsAction: 'Needs action',
  docking: 'Docking',
  awaitingReview: 'Awaiting review',
  waitingOnYou: 'Waiting on you',
  unassigned: 'Unassigned',
  notScheduled: 'Not scheduled',
  inheritProject: 'Inherit mission default',
  inheritFeature: 'Inherit from objective',
} as const

/** Approval bar and astronaut console actions (AGENT_BRIEF §5, §6). */
export const actionLabels = {
  approveLand: 'Approve & land',
  requestChanges: 'Request changes',
  takeOver: 'Take over',
  shortcuts: 'A approve · R request changes · T take over',
  pause: 'Pause',
  resume: 'Resume',
  redirect: 'Redirect',
  changeAutonomy: 'Autonomy',
  recall: 'Recall to Hangar',
  reply: 'Send',
  editTask: 'Edit payload',
  editFeature: 'Edit objective',
  archive: 'Archive',
  cancel: 'Cancel',
  save: 'Save changes',
  createFeature: 'Create objective',
  createTask: 'Create payload',
  planWithAgent: 'Plan with an astronaut',
  addCriterion: 'Add criterion',
  addSubtask: 'Add module',
  linkPr: 'Link PR',
  addExisting: 'Add existing',
  backToBoard: 'Back to board',
} as const

/** Payload drawer tabs (PayloadDetail.dc.html). */
export const taskTabLabels = {
  brief: 'Brief',
  subtasks: 'Modules',
  log: 'Flight log',
  cargo: 'Cargo',
  trace: 'Astronaut trace',
} as const

/** Shared create/edit form text (NewObjective, ObjectiveEditor mockups). */
export const formLabels = {
  feature: {
    eyebrowNew: 'New objective',
    eyebrowEdit: 'Objective',
    titleNew: 'New objective',
    titleEdit: 'Edit objective',
    intro: 'An objective groups the payloads that deliver one outcome. You can add payloads and success criteria after creating it.',
    nameLabel: 'Name',
    namePlaceholder: 'e.g. Realtime sync',
    descPlaceholder: 'What does landing this objective achieve?',
    parentLabel: 'Mission phase',
    peopleLabel: 'Owner',
    criteriaHeading: 'Success criteria',
    criteriaHelp: 'The objective lands when every criterion is met and every payload has landed.',
    autonomyLegend: 'Autonomy for new payloads in this objective',
    sideHeading: 'Payloads',
    progressLabel: 'Landed',
  },
  task: {
    eyebrowNew: 'New payload',
    eyebrowEdit: 'Payload',
    titleNew: 'New payload',
    titleEdit: 'Edit payload',
    intro: 'A payload is one piece of work. It waits in the Hangar until it is scheduled into a launch window.',
    nameLabel: 'Title',
    namePlaceholder: 'e.g. Retry cap for reconnects',
    descPlaceholder: 'What needs to be done, and how will we know it is done?',
    parentLabel: 'Objective',
    peopleLabel: 'Assignee',
    criteriaHeading: 'Acceptance criteria',
    criteriaHelp: 'The payload docks for review when every criterion is met.',
    autonomyLegend: 'Autonomy for this payload',
    sideHeading: 'Modules',
    progressLabel: 'Done',
  },
  description: 'Description',
  descriptionOptional: 'Description (optional)',
  sprint: 'Target launch window',
  estimate: 'Estimate (points)',
  priority: 'Priority',
  tags: 'Tags',
  tagsOptional: 'Tags (optional)',
} as const

/** Planner steps and screens (AGENT_BRIEF §8). */
export const plannerLabels = {
  steps: ['Briefing', 'Review doc', 'Review payloads', 'Done'],
  stepsLabel: 'Planning steps',
  saveAndExit: 'Save draft and exit',
  briefingAgent: 'Briefing astronaut',
  planningAgent: 'Planning astronaut',
  newFeature: 'New objective',
  draftSuffix: '(draft)',
  skipQuestion: 'Skip question',
  draftNow: 'Draft the doc now',
  sendAnswer: 'Send answer',
  yourAnswer: 'Your answer',
  approveDoc: 'Approve and plan payloads',
  denyDoc: 'Deny with feedback',
  editDoc: 'Edit the doc myself',
  approveTask: 'Approve and add to Hangar',
  editTask: 'Edit before approving',
  denyTask: 'Deny with feedback',
  finishReview: 'Finish review',
  openBoard: 'Open the board',
  keepReviewing: 'Keep reviewing payloads',
  viewFeature: 'View the objective',
  docReasons: ['Too broad', 'Missing detail', 'Wrong scope', 'Wrong approach', 'Other'],
  taskReasons: ['Too big', 'Unclear criteria', 'Duplicate', 'Wrong objective', 'Other'],
  docOutcomes: {
    revise: { label: 'Send back for revision', description: 'The astronaut may ask a few follow-up questions, then redraft.' },
    discard: { label: 'Discard this objective', description: 'Nothing is created and the draft is archived.' },
  },
  taskOutcomes: {
    revise: { label: 'Revise this payload', description: 'It returns to the queue marked "Revised" with a note on what changed.' },
    discard: { label: 'Drop this payload', description: 'It is removed from the plan and nothing is added to the board.' },
  },
} as const

/** Settings section names (Settings.dc.html). */
export const settingsLabels = {
  account: 'Account',
  github: 'GitHub',
  providers: 'LLM providers',
  providersHelp: 'OpenAI-compatible endpoints your astronauts and test runs can use. Keys are stored encrypted and never shown again.',
  profile: 'Profile',
  autonomy: 'Astronauts and autonomy',
  roles: 'Operators and roles',
  notifications: 'Notifications',
  integrations: 'Integrations',
  defaultAutonomy: 'Default autonomy',
  defaultAutonomyHelp: 'Applies to new astronauts. You can override it per astronaut or per payload.',
  freeFlightGrantedBy: 'Who can grant Free flight',
  grantLead: 'Flight director only',
  grantAny: 'Any operator member',
  agentTaskCap: 'Concurrent astronaut payloads',
  approvalRules: 'Approval rules',
  integrationsHelp: 'The AI company that powers your astronauts. Applies to every payload unless overridden.',
  apiKey: 'API key',
  changePassword: 'Change password',
  agents: 'Agents',
  agentsHelp: 'The LLM agents that plan your features. Each uses one of your providers.',
  review: 'Review plan docs before tasks are made',
  reviewHelp: 'Off: a finished plan doc is approved without review.',
  saveAgents: 'Save agent settings',
} as const

/** The LLM agents people configure: name and what each does. */
export const llmAgentLabels = {
  interviewer: { name: 'Interviewer', description: 'Asks about a feature one question at a time and writes its plan doc.' },
  taskAgent: { name: 'Task agent', description: 'Turns an approved plan doc into tasks with tests.' },
  decision: { name: 'Decision agent', description: 'Picks which workflow runs each test.' },
  chat: { name: 'Check agent', description: 'Runs LLM checks, using the browser and other tools.' },
} as const

export const autonomyDescriptions: Record<Autonomy, string> = {
  0: 'Approve the plan and the output.',
  1: 'Acts freely, output waits for approval.',
  2: 'Lands low-risk payloads on its own.',
}

export const priorityLabels: Record<Priority, string> = {
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  critical: 'Critical',
}

export const featureStatusLabels: Record<FeatureStatus, string> = {
  planned: 'Planned',
  active: 'Active',
  landed: 'Landed',
  archived: 'Archived',
}

export const traceKindLabels: Record<TraceKind, string> = {
  plan: 'Plan',
  read: 'Tool call · read',
  edit: 'Tool call · edit',
  run: 'Tool call · run',
  result: 'Result',
  hail: 'Hail',
  docking: 'Docking',
}

export const activityVerbLabels: Record<ActivityVerb, string> = {
  moved: 'moved',
  created: 'created',
  commented: 'commented on',
  hailed: 'hailed on',
  answered: 'answered a hail on',
  'opened-pr': 'opened a PR for',
  approved: 'approved',
  'requested-changes': 'requested changes on',
  'took-over': 'took over',
  recalled: 'recalled',
  blocked: 'lost comms on',
}

export const draftStatusLabels: Record<DraftStatus, string> = {
  briefing: 'Briefing',
  'needs-review': 'Needs your review',
  approved: 'Approved',
  denied: 'Sent back',
  discarded: 'Discarded',
}

export const topicStateLabels: Record<DocTopicState, string> = {
  covered: 'Covered',
  asking: 'Asking now',
  'not-yet': 'Not yet',
}

export const proposalStatusLabels: Record<ProposalStatus, string> = {
  'needs-review': 'Needs review',
  approved: 'Approved',
  'approved-edited': 'Approved · edited',
  'sent-back': 'Sent back',
  dropped: 'Dropped',
  revised: 'Revised',
}

// Each column shows its Space name with the plain status word beside it.
export const columnLabels: Record<ColumnKey, { name: string; plain: string }> = {
  backlog: { name: 'Hangar', plain: 'Backlog' },
  'in-progress': { name: 'In orbit', plain: 'In Progress' },
  testing: { name: 'Static fire', plain: 'Testing' },
  'user-check': { name: 'Docking', plain: 'User Check' },
  closed: { name: 'Landed', plain: 'Closed' },
}

export const taskStatusLabels: Record<TaskStatus, string> = {
  queued: 'Queued',
  ready: 'Ready',
  working: 'Working',
  hail: 'Hail',
  blocked: 'Comms down',
  'awaiting-approval': 'Approve?',
  'in-review': 'In review',
  landed: 'Landed',
}

export const autonomyLabels: Record<Autonomy, string> = {
  0: 'Tethered',
  1: 'Guided',
  2: 'Free flight',
}

export const agentStateLabels: Record<AgentState, string> = {
  idle: 'Idle',
  planning: 'Planning',
  working: 'In orbit',
  hail: 'Hail',
  docking: 'Docking',
  blocked: 'Comms down',
}

export const roleLabels: Record<UserRole, string> = {
  lead: 'Flight director',
  member: 'Operator',
}

/** A task's latest test run, as the board card and drawer show it. */
export const runStatusLabels: Record<RunStatus, string> = {
  queued: 'Test queued',
  running: 'Testing',
  passed: 'Tests passed',
  failed: 'Tests failed',
  error: 'Test error',
  cancelled: 'Test cancelled',
}
