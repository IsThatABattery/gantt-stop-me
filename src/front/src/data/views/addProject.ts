// Add a GitHub repository as a project, or find the one added before
// (POST /api/projects, docs/api/board.md).

import { useQuery } from '@tanstack/react-query'
import { api, useApiMutation } from '../api/client'
import type { Project } from '../api/types'
import { keys } from '../keys'

const addProject = (url: string) => api<Project>('POST', '/projects', { url })

export const useAddProject = () => useApiMutation(addProject, [keys.projects])

/** The projects the user has added or opened and can still see, most recently used first (GET /api/projects). */
export const useMyProjects = () =>
  useQuery({ queryKey: keys.projects, queryFn: () => api<{ projects: Project[] }>('GET', '/projects').then((r) => r.projects), retry: false })
