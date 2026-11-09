import type { FormOptions, ItemInput } from '../../data/views/forms'
import type { Feature, Task } from '../../data/types'

// Form state for the shared Basics fields, and conversions to and from the data.

export type Kind = 'feature' | 'task'

/** What the Basics fields edit. Estimate stays text so the field can be empty. */
export interface BasicsValue extends Omit<ItemInput, 'estimate'> {
  estimate: string
}

export const toInput = (v: BasicsValue): ItemInput => ({ ...v, estimate: v.estimate.trim() === '' ? 0 : Number(v.estimate) })

/** Client-side checks, so errors show before anything is sent. */
export function basicsErrors(kind: Kind, v: BasicsValue): Partial<Record<'name' | 'estimate', string>> {
  const errors: Partial<Record<'name' | 'estimate', string>> = {}
  if (!v.name.trim()) errors.name = kind === 'feature' ? 'Give the feature a name.' : 'Give the task a title.'
  const est = toInput(v).estimate
  if (kind === 'task' && (!Number.isFinite(est) || est < 0)) errors.estimate = 'Estimate must be 0 or more.'
  return errors
}

export function emptyBasics(kind: Kind, options: FormOptions, defaults: { userId: string; sprintId: string | null }): BasicsValue {
  const parents = kind === 'feature' ? options.epics : openFeatures(options.features)
  return {
    name: '',
    description: '',
    parentId: parents[0]?.id ?? '',
    people: kind === 'feature' ? { kind: 'user', id: defaults.userId } : null,
    sprintId: defaults.sprintId,
    tags: [],
    estimate: '',
    priority: 'medium',
  }
}

export const featureBasics = (f: Feature): BasicsValue => ({
  name: f.name,
  description: f.description,
  parentId: f.epicId,
  people: f.ownerId ? { kind: 'user', id: f.ownerId } : null,
  sprintId: f.sprintId,
  tags: f.tags,
  estimate: '',
  priority: 'medium',
})

export const taskBasics = (t: Task): BasicsValue => ({
  name: t.title,
  description: t.description,
  parentId: t.featureId,
  people: t.assignee,
  sprintId: t.sprintId,
  tags: t.tags,
  estimate: String(t.estimate),
  priority: t.priority,
})

const openFeatures = (features: Feature[]) =>
  features.filter((f) => f.status !== 'archived').sort((a, b) => a.name.localeCompare(b.name))
