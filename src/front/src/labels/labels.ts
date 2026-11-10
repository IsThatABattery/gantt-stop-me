import type { RunStatus } from '../data/api/types'
import type { ColumnKey } from '../data/types'

// Display labels. Plain words are the default voice (src/front/PRODUCT.md, docs/glossary.md):
// Project, Task, Backlog, Testing, Agent. Code, data and URLs use the same plain
// names. The app's shared words come from here, so a themed label set can be
// swapped in later; some views still write their own sentences. Keep the keys
// and shapes stable: every view reads them.
export const labels = {
  appName: 'GanttStopMe',

  project: 'Project',
  projects: 'Projects',
  task: 'Task',
  tasks: 'Tasks',
  backlog: 'Backlog',
  board: 'Board',
  settings: 'Settings',
  automation: 'Automation',
  plans: 'Plan a feature',

  allProjects: 'All projects',
  addProject: 'Add a project',
  testsRunning: 'Tests running',
  linkGitHub: 'Link GitHub',
  newUser: 'New here?',
  requestAccess: 'Create an account',
  newTask: 'New task',
  awaitingApproval: 'Awaiting approval',
  assignedToYou: 'Assigned to you',
  needsAttention: 'Needs attention',
  unassigned: 'Unassigned',
  /** The signed-in person, on their avatar */
  member: 'Member',
} as const

/** Buttons that act on a task: approve, request changes, create, edit, save. */
export const actionLabels = {
  approveLand: 'Approve and merge',
  requestChanges: 'Request changes',
  editTask: 'Edit task',
  cancel: 'Cancel',
  save: 'Save changes',
  createTask: 'Create task',
  backToBoard: 'Back to board',
} as const

/** The task create dialog and edit page. */
export const formLabels = {
  task: {
    titleNew: 'New task',
    titleEdit: 'Edit task',
    nameLabel: 'Title',
    namePlaceholder: 'e.g. Retry cap for reconnects',
    descPlaceholder: 'What needs to be done, and how will we know it is done?',
  },
  description: 'Description',
  descriptionOptional: 'Description (optional)',
} as const

/** Settings section names and help. */
export const settingsLabels = {
  account: 'Account',
  appearance: 'Appearance',
  appearanceHelp: 'Applies right away, and is saved in this browser.',
  theme: 'Theme',
  github: 'GitHub',
  providers: 'LLM providers',
  providersHelp: 'OpenAI-compatible endpoints your agents and test runs can use. Keys are stored encrypted and never shown again.',
  apiKey: 'API key',
  changePassword: 'Change password',
  setPassword: 'Set a password',
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

/** A task whose issue is closed */
export const closedTaskLabel = 'Closed'

// Each column's name. name and plain are the same plain word while plain words
// are the default; a themed label set gives name its own word and keeps plain,
// which is then shown beside it. Show name alone when the two are equal.
export const columnLabels: Record<ColumnKey, { name: string; plain: string }> = {
  backlog: { name: 'Backlog', plain: 'Backlog' },
  'in-progress': { name: 'In Progress', plain: 'In Progress' },
  testing: { name: 'Testing', plain: 'Testing' },
  'user-check': { name: 'User Check', plain: 'User Check' },
  closed: { name: 'Closed', plain: 'Closed' },
}

/**
 * A task's latest test run, as the board card and drawer show it. A failed run
 * found broken code; a runner error means the run itself broke (docs/glossary.md).
 */
export const runStatusLabels: Record<RunStatus, string> = {
  queued: 'Tests queued',
  running: 'Tests running',
  passed: 'Tests passed',
  failed: 'Tests failed',
  error: 'Runner error',
  cancelled: 'Tests cancelled',
}
