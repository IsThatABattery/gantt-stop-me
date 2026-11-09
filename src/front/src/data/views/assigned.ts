// The open tasks assigned to the user, by project (GET /api/me/tasks, docs/api/board.md).

import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import type { AssignedGroup } from '../api/types'
import { keys } from '../keys'

const live = (groups: AssignedGroup[] | undefined) =>
  !!groups?.some((g) => g.tasks.some((t) => t.runStatus === 'queued' || t.runStatus === 'running'))

/** Polls fast while one of them is queued or testing, so the rail follows the run, and slowly otherwise. */
export const useAssignedTasks = () =>
  useQuery({
    queryKey: keys.assigned,
    queryFn: () => api<{ projects: AssignedGroup[] }>('GET', '/me/tasks').then((r) => r.projects),
    refetchInterval: (q) => (live(q.state.data) ? 3000 : 30000),
  })
