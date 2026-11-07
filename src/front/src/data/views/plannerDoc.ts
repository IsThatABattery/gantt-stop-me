// Planner steps 1–2: briefing and doc review (feature/frontend-planner-doc).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay, NotFoundError } from '../mock/store'
import type { FeatureDraft, ID } from '../types'

export function getDraft(draftId: ID): Promise<FeatureDraft> {
  const d = db.featureDrafts.find((x) => x.id === draftId)
  return d ? delay(d) : Promise.reject(new NotFoundError('Draft'))
}

export const useDraft = (draftId: ID) => useQuery({ queryKey: keys.draft(draftId), queryFn: () => getDraft(draftId) })
