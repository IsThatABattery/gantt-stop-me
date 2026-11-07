import { useQuery } from '@tanstack/react-query'
import * as api from './api'
import type { ID } from './types'

export const useWorkspace = () => useQuery({ queryKey: ['workspace'], queryFn: api.getWorkspace })

export const useUser = (id: ID | undefined) =>
  useQuery({ queryKey: ['user', id], queryFn: () => api.getUser(id!), enabled: !!id })

export const useProjects = () => useQuery({ queryKey: ['projects'], queryFn: api.listProjects })

export const useProject = (projectId: ID) =>
  useQuery({ queryKey: ['project', projectId], queryFn: () => api.getProject(projectId) })

export const useBoard = (projectId: ID) =>
  useQuery({ queryKey: ['board', projectId], queryFn: () => api.getBoard(projectId) })
