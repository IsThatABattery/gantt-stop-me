// Mission log (feature/frontend-activity).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay } from '../mock/store'
import type { ActivityEvent, Agent, ID, Task, User } from '../types'

/** Newest first. */
export function listActivity(projectId: ID): Promise<ActivityEvent[]> {
  return delay(db.activityEvents.filter((e) => e.projectId === projectId).sort((a, b) => b.at.localeCompare(a.at)))
}

export const useActivity = (projectId: ID) => useQuery({ queryKey: keys.activity(projectId), queryFn: () => listActivity(projectId) })

/** Who and what the events point at, so the feed can show names and payload keys. */
export interface ActivityContext {
  users: User[]
  agents: Agent[]
  tasks: Pick<Task, 'id' | 'key' | 'title'>[]
}

export function getActivityContext(projectId: ID): Promise<ActivityContext> {
  return delay({
    users: db.users,
    agents: db.agents,
    tasks: db.tasks.filter((t) => t.projectId === projectId).map(({ id, key, title }) => ({ id, key, title })),
  })
}

// Sits under keys.activity, so any write that refreshes the log refreshes this too.
export const useActivityContext = (projectId: ID) =>
  useQuery({ queryKey: [...keys.activity(projectId), 'context'], queryFn: () => getActivityContext(projectId) })
