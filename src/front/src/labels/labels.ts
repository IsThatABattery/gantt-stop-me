import type { AgentState, Autonomy, ColumnKey, TaskStatus, UserRole } from '../data/types'

// Display labels for the Space theme. Code, data and URLs use plain names;
// every themed word the user sees comes from here, so another theme can be
// added later by swapping this object.
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

  allProjects: 'All missions',
  newProject: 'New mission',
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
} as const

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
