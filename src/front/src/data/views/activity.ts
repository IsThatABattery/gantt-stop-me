// Mission log (feature/frontend-activity).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay } from '../mock/store'
import type { ActivityEvent, ID } from '../types'

/** Newest first. */
export function listActivity(projectId: ID): Promise<ActivityEvent[]> {
  return delay(db.activityEvents.filter((e) => e.projectId === projectId).sort((a, b) => b.at.localeCompare(a.at)))
}

export const useActivity = (projectId: ID) => useQuery({ queryKey: keys.activity(projectId), queryFn: () => listActivity(projectId) })
