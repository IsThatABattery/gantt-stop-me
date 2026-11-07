// Settings (feature/frontend-settings).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay } from '../mock/store'
import type { WorkspaceSettings } from '../types'

export function getSettings(): Promise<WorkspaceSettings> {
  return delay(db.settings)
}

export const useSettings = () => useQuery({ queryKey: keys.settings, queryFn: getSettings })
