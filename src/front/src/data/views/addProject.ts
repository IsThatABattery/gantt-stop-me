// Add a GitHub repository as a project, or find the one added before
// (POST /api/projects, docs/board.md).

import { api, useApiMutation } from '../api/client'
import type { Project } from '../api/types'

export const addProject = (url: string) => api<Project>('POST', '/projects', { url })

export const useAddProject = () => useApiMutation(addProject, [])
