// Mission control board (feature/frontend-board).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay, NotFoundError } from '../mock/store'
import { COLUMN_ORDER, type Board, type ID } from '../types'

export function getBoard(projectId: ID): Promise<Board> {
  const project = db.projects.find((p) => p.id === projectId)
  if (!project) return Promise.reject(new NotFoundError('Project'))
  const tasks = db.tasks.filter((t) => t.projectId === projectId).sort((a, b) => a.position - b.position)
  return delay({
    project,
    sprint: db.sprints.find((s) => s.id === project.activeSprintId) ?? null,
    columns: COLUMN_ORDER.map((key) => ({ key, tasks: tasks.filter((t) => t.column === key) })),
    epics: db.epics.filter((e) => e.projectId === projectId),
    features: db.features.filter((f) => f.projectId === projectId),
    users: db.users,
    agents: db.agents,
  })
}

export const useBoard = (projectId: ID) => useQuery({ queryKey: keys.board(projectId), queryFn: () => getBoard(projectId) })
