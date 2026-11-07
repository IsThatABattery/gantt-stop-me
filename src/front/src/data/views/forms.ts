// Create and edit forms for features and tasks (feature/frontend-forms).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay, NotFoundError } from '../mock/store'
import type { Agent, Epic, Feature, ID, Sprint, User } from '../types'

/** Choices for the form's selects. */
export interface FormOptions {
  epics: Epic[]
  features: Feature[]
  users: User[]
  agents: Agent[]
  sprints: Sprint[]
}

export function getFormOptions(projectId: ID): Promise<FormOptions> {
  const project = db.projects.find((p) => p.id === projectId)
  if (!project) return Promise.reject(new NotFoundError('Project'))
  return delay({
    epics: db.epics.filter((e) => e.projectId === projectId),
    features: db.features.filter((f) => f.projectId === projectId),
    users: db.users.filter((u) => project.memberIds.includes(u.id)),
    agents: db.agents,
    sprints: db.sprints.filter((s) => s.projectId === projectId),
  })
}

export function getFeature(projectId: ID, featureId: ID): Promise<Feature> {
  const f = db.features.find((x) => x.id === featureId && x.projectId === projectId)
  return f ? delay(f) : Promise.reject(new NotFoundError('Feature'))
}

export const useFormOptions = (projectId: ID) =>
  useQuery({ queryKey: keys.formOptions(projectId), queryFn: () => getFormOptions(projectId) })

export const useFeature = (projectId: ID, featureId: ID) =>
  useQuery({ queryKey: keys.feature(projectId, featureId), queryFn: () => getFeature(projectId, featureId) })
