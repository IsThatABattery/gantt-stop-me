// Astronaut console (feature/frontend-agents).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay } from '../mock/store'
import type { Agent, Hail, ID, Task, TraceStep } from '../types'

export interface AgentRow {
  agent: Agent
  task: Task | null
  openHail: Hail | null
  trace: TraceStep[]
}

/** Agents working on this project's tasks, with their current task. */
export function listAgents(projectId: ID): Promise<AgentRow[]> {
  const rows = db.agents.map((agent) => {
    const task = db.tasks.find((t) => t.id === agent.currentTaskId && t.projectId === projectId) ?? null
    return {
      agent,
      task,
      openHail: db.hails.find((h) => h.agentId === agent.id && h.taskId === task?.id && !h.answer) ?? null,
      trace: db.traceSteps.filter((s) => s.agentId === agent.id && s.taskId === task?.id),
    }
  })
  return delay(rows.filter((r) => r.task))
}

export const useAgents = (projectId: ID) => useQuery({ queryKey: keys.agents(projectId), queryFn: () => listAgents(projectId) })
