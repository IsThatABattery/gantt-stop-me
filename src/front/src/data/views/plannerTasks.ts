// Planner steps 3–4: payload review and done (feature/frontend-planner-tasks).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay } from '../mock/store'
import type { ID, TaskProposal } from '../types'

export function listProposals(draftId: ID): Promise<TaskProposal[]> {
  return delay(db.taskProposals.filter((p) => p.draftId === draftId).sort((a, b) => a.order - b.order))
}

export const useProposals = (draftId: ID) =>
  useQuery({ queryKey: keys.proposals(draftId), queryFn: () => listProposals(draftId) })
