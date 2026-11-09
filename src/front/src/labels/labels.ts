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

// Display labels. Plain words are the default voice (PRODUCT.md, GLOSSARY.md):
// Project, Task, Backlog, Testing, Agent. Code, data and URLs use the same plain
// names; every word the user sees comes from here, so a themed label set (the
// old space-mission one, say) can be swapped in later without touching a view.
// Keep the keys and shapes stable: every view reads them.
export const labels = {
  appName: 'GanttStopMe',

  workspace: 'Workspace',
  project: 'Project',
  projects: 'Projects',
  epic: 'Epic',
  feature: 'Feature',
  features: 'Features',
  task: 'Task',
  tasks: 'Tasks',
  subtask: 'Subtask',
  backlog: 'Backlog',
  sprint: 'Sprint',
  sprints: 'Sprints',
  user: 'Person',
  users: 'People',
  agent: 'Agent',
  agents: 'Agents',
  board: 'Board',
  activity: 'Activity',
  retro: 'Retro',
  settings: 'Settings',
  automation: 'Automation',
  plans: 'Plan a feature',

  allProjects: 'All projects',
  addProject: 'Add a project',
  newProject: 'New project',
  testsRunning: 'Tests running',
  linkGitHub: 'Link GitHub',
  newProjectHint: 'Start from scratch or import a backlog',
  selectProject: 'Select a project',
  searchProjects: 'Search projects',
  newUser: 'New here?',
  requestAccess: 'Create an account',
  needsYou: 'Needs you',
  reviewNow: 'Review now',
  tasksLanded: 'Tasks closed',
  newFeature: 'New feature',
  newTask: 'New task',
  manageFeatures: 'Manage features',
  allFeatures: 'All features',
  tMinus: 'Ends in',
  awaitingApproval: 'Awaiting approval',
  blocked: 'Needs attention',
  agentsActive: 'Agents active',
  hail: 'Question',
  hails: 'Questions',
  sprintName: (n: number) => `Sprint ${n}`,
  inSprint: (n: number) => `In sprint ${n}`,
  nextSprint: (date: string) => `Next: ${date}`,
  betweenSprints: 'Between sprints',
  lead: 'Lead',
  operatorsOnly: 'People only',
  agentsOnly: 'Agents only',
  needsAction: 'Needs action',
  docking: 'User Check',
  awaitingReview: 'Awaiting review',
  assignedToYou: 'Assigned to you',
  needsAttention: 'Needs attention',
  waitingOnYou: 'Waiting on you',
  unassigned: 'Unassigned',
  notScheduled: 'Not scheduled',
  inheritProject: 'Inherit project default',
  inheritFeature: 'Inherit from feature',
} as const

/** Approval bar and agent console actions (AGENT_BRIEF §5, §6). */
export const actionLabels = {
  approveLand: 'Approve and merge',
  requestChanges: 'Request changes',
  takeOver: 'Take over',
  shortcuts: 'A approve · R request changes · T take over',
  pause: 'Pause',
  resume: 'Resume',
  redirect: 'Redirect',
  changeAutonomy: 'Autonomy',
  recall: 'Recall to Backlog',
  reply: 'Send',
  editTask: 'Edit task',
  editFeature: 'Edit feature',
  archive: 'Archive',
  cancel: 'Cancel',
  save: 'Save changes',
  createFeature: 'Create feature',
  createTask: 'Create task',
  planWithAgent: 'Plan with an agent',
  addCriterion: 'Add criterion',
  addSubtask: 'Add subtask',
  linkPr: 'Link PR',
  addExisting: 'Add existing',
  backToBoard: 'Back to board',
} as const

/** Task drawer tabs (PayloadDetail.dc.html). */
export const taskTabLabels = {
  brief: 'Brief',
  subtasks: 'Subtasks',
  log: 'Log',
  cargo: 'Files',
  trace: 'Agent trace',
} as const

/** Shared create/edit form text (NewObjective, ObjectiveEditor mockups). */
export const formLabels = {
  feature: {
    titleNew: 'New feature',
    titleEdit: 'Edit feature',
    intro: 'A feature groups the tasks that deliver one outcome. You can add tasks and success criteria after creating it.',
    nameLabel: 'Name',
    namePlaceholder: 'e.g. Realtime sync',
    descPlaceholder: 'What does finishing this feature achieve?',
    parentLabel: 'Epic',
    peopleLabel: 'Owner',
    criteriaHeading: 'Success criteria',
    criteriaHelp: 'The feature is done when every criterion is met and every task is closed.',
    autonomyLegend: 'Autonomy for new tasks in this feature',
    sideHeading: 'Tasks',
    progressLabel: 'Closed',
  },
  task: {
    titleNew: 'New task',
    titleEdit: 'Edit task',
    intro: 'A task is one piece of work. It waits in the Backlog until it is scheduled into a sprint.',
    nameLabel: 'Title',
    namePlaceholder: 'e.g. Retry cap for reconnects',
    descPlaceholder: 'What needs to be done, and how will we know it is done?',
    parentLabel: 'Feature',
    peopleLabel: 'Assignee',
    criteriaHeading: 'Acceptance criteria',
    criteriaHelp: 'The task moves to User Check when every criterion is met.',
    autonomyLegend: 'Autonomy for this task',
    sideHeading: 'Subtasks',
    progressLabel: 'Done',
  },
  description: 'Description',
  descriptionOptional: 'Description (optional)',
  sprint: 'Target sprint',
  estimate: 'Estimate (points)',
  priority: 'Priority',
  tags: 'Tags',
  tagsOptional: 'Tags (optional)',
} as const

/** Planner steps and screens (AGENT_BRIEF §8). */
export const plannerLabels = {
  steps: ['Interview', 'Review doc', 'Review tasks', 'Done'],
  stepsLabel: 'Planning steps',
  saveAndExit: 'Save draft and exit',
  briefingAgent: 'Interviewer agent',
  planningAgent: 'Planning agent',
  newFeature: 'New feature',
  draftSuffix: '(draft)',
  skipQuestion: 'Skip question',
  draftNow: 'Draft the doc now',
  sendAnswer: 'Send answer',
  yourAnswer: 'Your answer',
  approveDoc: 'Approve and plan tasks',
  denyDoc: 'Deny with feedback',
  editDoc: 'Edit the doc myself',
  approveTask: 'Approve and add to Backlog',
  editTask: 'Edit before approving',
  denyTask: 'Deny with feedback',
  finishReview: 'Finish review',
  openBoard: 'Open the board',
  keepReviewing: 'Keep reviewing tasks',
  viewFeature: 'View the feature',
  docReasons: ['Too broad', 'Missing detail', 'Wrong scope', 'Wrong approach', 'Other'],
  taskReasons: ['Too big', 'Unclear criteria', 'Duplicate', 'Wrong feature', 'Other'],
  docOutcomes: {
    revise: { label: 'Send back for revision', description: 'The agent may ask a few follow-up questions, then redraft.' },
    discard: { label: 'Discard this feature', description: 'Nothing is created and the draft is archived.' },
  },
  taskOutcomes: {
    revise: { label: 'Revise this task', description: 'It returns to the queue marked "Revised" with a note on what changed.' },
    discard: { label: 'Drop this task', description: 'It is removed from the plan and nothing is added to the board.' },
  },
} as const

/** Settings section names (Settings.dc.html). */
export const settingsLabels = {
  account: 'Account',
  appearance: 'Appearance',
  appearanceHelp: 'Applies right away, and is saved in this browser.',
  theme: 'Theme',
  github: 'GitHub',
  providers: 'LLM providers',
  providersHelp: 'OpenAI-compatible endpoints your agents and test runs can use. Keys are stored encrypted and never shown again.',
  profile: 'Profile',
  autonomy: 'Agents and autonomy',
  roles: 'People and roles',
  notifications: 'Notifications',
  integrations: 'Integrations',
  defaultAutonomy: 'Default autonomy',
  defaultAutonomyHelp: 'Applies to new agents. You can override it per agent or per task.',
  freeFlightGrantedBy: 'Who can make an agent Autonomous',
  grantLead: 'The lead only',
  grantAny: 'Any member',
  agentTaskCap: 'Concurrent agent tasks',
  approvalRules: 'Approval rules',
  integrationsHelp: 'The AI company that powers your agents. Applies to every task unless overridden.',
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
  2: 'Merges low-risk tasks on its own.',
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
  landed: 'Done',
  archived: 'Archived',
}

export const traceKindLabels: Record<TraceKind, string> = {
  plan: 'Plan',
  read: 'Tool call · read',
  edit: 'Tool call · edit',
  run: 'Tool call · run',
  result: 'Result',
  hail: 'Question',
  docking: 'Review',
}

export const activityVerbLabels: Record<ActivityVerb, string> = {
  moved: 'moved',
  created: 'created',
  commented: 'commented on',
  hailed: 'asked a question on',
  answered: 'answered a question on',
  'opened-pr': 'opened a PR for',
  approved: 'approved',
  'requested-changes': 'requested changes on',
  'took-over': 'took over',
  recalled: 'recalled',
  blocked: 'flagged for attention',
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

// Each column's name. name and plain are the same plain word while plain words
// are the default; a themed label set gives name its own word and keeps plain,
// which is then shown beside it. Show name alone when the two are equal.
/** What each column is for, in a few words, under its tab. Columns the dictionary doesn't know show no line. */
export const columnPurposeLabels: Partial<Record<string, string>> = {
  backlog: 'Waiting to start',
  'in-progress': 'Being built',
  testing: 'Tests run on the linked PR',
  'user-check': 'A person checks and approves',
  closed: 'Merged and closed',
}

/** A task with no test run yet */
export const noTestRunLabel = 'No test run'

export const columnLabels: Record<ColumnKey, { name: string; plain: string }> = {
  backlog: { name: 'Backlog', plain: 'Backlog' },
  'in-progress': { name: 'In Progress', plain: 'In Progress' },
  testing: { name: 'Testing', plain: 'Testing' },
  'user-check': { name: 'User Check', plain: 'User Check' },
  closed: { name: 'Closed', plain: 'Closed' },
}

export const taskStatusLabels: Record<TaskStatus, string> = {
  queued: 'Queued',
  ready: 'Ready',
  working: 'Working',
  hail: 'Question',
  blocked: 'Needs attention',
  'awaiting-approval': 'Awaiting approval',
  'in-review': 'In review',
  landed: 'Closed',
}

export const autonomyLabels: Record<Autonomy, string> = {
  0: 'Supervised',
  1: 'Guided',
  2: 'Autonomous',
}

export const agentStateLabels: Record<AgentState, string> = {
  idle: 'Idle',
  planning: 'Planning',
  working: 'Working',
  hail: 'Question',
  docking: 'In review',
  blocked: 'Needs attention',
}

export const roleLabels: Record<UserRole, string> = {
  lead: 'Lead',
  member: 'Member',
}

/**
 * A task's latest test run, as the board card and drawer show it. A failed run
 * found broken code; a runner error means the run itself broke (GLOSSARY.md).
 */
export const runStatusLabels: Record<RunStatus, string> = {
  queued: 'Tests queued',
  running: 'Tests running',
  passed: 'Tests passed',
  failed: 'Tests failed',
  error: 'Runner error',
  cancelled: 'Tests cancelled',
}
