// Astronaut console (feature/frontend-agents).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay, mustFind, newId, signedInUserId, useMockMutation } from '../mock/store'
import type { ActivityVerb, Agent, Autonomy, Hail, ID, Task, TraceStep, WorkspaceSettings } from '../types'

export interface AgentRow {
  agent: Agent
  task: Task | null
  openHail: Hail | null
  trace: TraceStep[]
}

/**
 * Agents on this project: working on one of its tasks, or still assigned
 * to one that hasn't landed (e.g. just recalled to the Hangar).
 */
export function listAgents(projectId: ID): Promise<AgentRow[]> {
  const mine = db.tasks.filter((t) => t.projectId === projectId)
  const rows = db.agents
    .filter(
      (agent) =>
        mine.some((t) => t.id === agent.currentTaskId) ||
        mine.some((t) => t.assignee?.kind === 'agent' && t.assignee.id === agent.id && t.status !== 'landed'),
    )
    .map((agent) => {
      const task = mine.find((t) => t.id === agent.currentTaskId) ?? null
      return {
        agent,
        task,
        openHail: db.hails.find((h) => h.agentId === agent.id && h.taskId === task?.id && !h.answer) ?? null,
        trace: db.traceSteps.filter((s) => s.agentId === agent.id && s.taskId === task?.id),
      }
    })
  return delay(rows)
}

export const useAgents = (projectId: ID) => useQuery({ queryKey: keys.agents(projectId), queryFn: () => listAgents(projectId) })

/** Who may raise an astronaut to Free flight (workspace setting). */
export function getAutonomyPolicy(): Promise<Pick<WorkspaceSettings, 'freeFlightGrantedBy'>> {
  return delay({ freeFlightGrantedBy: db.settings.freeFlightGrantedBy })
}

export const useAutonomyPolicy = (projectId: ID) =>
  useQuery({ queryKey: [...keys.agents(projectId), 'policy'], queryFn: getAutonomyPolicy })

// ---- Writes ----

export class ConsoleError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ConsoleError'
  }
}

function write<T>(fn: () => T): Promise<T> {
  try {
    return delay(fn())
  } catch (e) {
    return Promise.reject(e)
  }
}

const findAgent = (agentId: ID) => mustFind(db.agents, (a) => a.id === agentId, 'Agent')

function logEvent(projectId: ID, taskId: ID | null, verb: ActivityVerb, detail: string) {
  db.activityEvents.push({
    id: newId('ev'),
    projectId,
    at: new Date().toISOString(),
    actor: { kind: 'user', id: signedInUserId },
    verb,
    taskId,
    detail,
    needsAction: false,
  })
}

interface AgentRef {
  projectId: ID
  agentId: ID
}

/** Answer a hail: the astronaut and its task go back to work. */
export function answerHail({ projectId, agentId, hailId, answer }: AgentRef & { hailId: ID; answer: string }): Promise<Hail> {
  return write(() => {
    const text = answer.trim()
    if (!text) throw new ConsoleError('Type an answer first.')
    const hail = mustFind(db.hails, (h) => h.id === hailId && h.agentId === agentId, 'Hail')
    if (hail.answer) throw new ConsoleError('This hail was already answered.')
    hail.answer = text
    hail.answeredBy = signedInUserId
    hail.answeredAt = new Date().toISOString()
    findAgent(agentId).state = 'working'
    const task = db.tasks.find((t) => t.id === hail.taskId)
    if (task) {
      task.status = 'working'
      task.version++
    }
    // The hail no longer waits on anyone.
    for (const e of db.activityEvents) if (e.taskId === hail.taskId && e.verb === 'hailed') e.needsAction = false
    logEvent(projectId, hail.taskId, 'answered', text)
    return hail
  })
}

export function setPaused({ agentId, paused }: AgentRef & { paused: boolean }): Promise<Agent> {
  return write(() => {
    const agent = findAgent(agentId)
    agent.paused = paused
    return agent
  })
}

/** No agent runtime yet, so a redirect is logged for the record. */
export function redirectAgent({ projectId, agentId, instruction }: AgentRef & { instruction: string }): Promise<Agent> {
  return write(() => {
    const text = instruction.trim()
    if (!text) throw new ConsoleError('Type an instruction first.')
    const agent = findAgent(agentId)
    logEvent(projectId, agent.currentTaskId, 'commented', `Redirected ${agent.name}: ${text}`)
    return agent
  })
}

export function setAutonomy({ agentId, autonomy }: AgentRef & { autonomy: Autonomy }): Promise<Agent> {
  return write(() => {
    const me = mustFind(db.users, (u) => u.id === signedInUserId, 'User')
    if (autonomy === 2 && db.settings.freeFlightGrantedBy === 'lead' && me.role !== 'lead') {
      throw new ConsoleError('Only the flight director can grant Free flight.')
    }
    const agent = findAgent(agentId)
    agent.autonomy = autonomy
    return agent
  })
}

/** Recall to Hangar: the task goes back to the Backlog and the astronaut stands down. */
export function recallAgent({ projectId, agentId }: AgentRef): Promise<Agent> {
  return write(() => {
    const agent = findAgent(agentId)
    const task = db.tasks.find((t) => t.id === agent.currentTaskId && t.projectId === projectId)
    if (!task) throw new ConsoleError(`${agent.name} has no payload to recall.`)
    const last = Math.max(0, ...db.tasks.filter((t) => t.projectId === projectId && t.column === 'backlog').map((t) => t.position))
    task.column = 'backlog'
    task.position = last + 1
    task.status = 'queued'
    task.blockedReason = null
    task.version++
    agent.currentTaskId = null
    agent.state = 'idle'
    agent.timeOnTaskMinutes = 0
    logEvent(projectId, task.id, 'recalled', `Recalled ${agent.name} to the Hangar`)
    return agent
  })
}

const refresh = (projectId: ID) => [keys.agents(projectId), keys.board(projectId), keys.activity(projectId), ['task', projectId]]

export const useAnswerHail = (projectId: ID) => useMockMutation(answerHail, refresh(projectId))
export const useSetPaused = (projectId: ID) => useMockMutation(setPaused, refresh(projectId))
export const useRedirectAgent = (projectId: ID) => useMockMutation(redirectAgent, refresh(projectId))
export const useSetAutonomy = (projectId: ID) => useMockMutation(setAutonomy, refresh(projectId))
export const useRecallAgent = (projectId: ID) => useMockMutation(recallAgent, refresh(projectId))
