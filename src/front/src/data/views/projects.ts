// Mission select (feature/frontend-projects).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay } from '../mock/store'
import type { ProjectSummary } from '../types'

export function listProjects(): Promise<ProjectSummary[]> {
  const list = db.projects.map((p) => ({
    ...p,
    stats: db.projectStats[p.id],
    activeSprint: db.sprints.find((s) => s.id === p.activeSprintId) ?? null,
    members: p.memberIds.map((id) => db.users.find((u) => u.id === id)).filter((u) => !!u),
  }))
  list.sort((a, b) => b.lastActivityAt.localeCompare(a.lastActivityAt))
  return delay(list)
}

export const useProjects = () => useQuery({ queryKey: keys.projects, queryFn: listProjects })
