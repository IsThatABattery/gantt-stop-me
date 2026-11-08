// Planner steps 3–4: payload review and done (feature/frontend-planner-tasks).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { keys } from '../keys'
import { db, delay, mustFind, newId, signedInUserId, useMockMutation } from '../mock/store'
import type { Agent, ColumnKey, Epic, Feature, FeedbackOutcome, ID, Task, TaskProposal, User } from '../types'

export function listProposals(draftId: ID): Promise<TaskProposal[]> {
  return delay(db.taskProposals.filter((p) => p.draftId === draftId).sort((a, b) => a.order - b.order))
}

export const useProposals = (draftId: ID) =>
  useQuery({ queryKey: keys.proposals(draftId), queryFn: () => listProposals(draftId) })

const findDraft = (draftId: ID) => mustFind(db.featureDrafts, (d) => d.id === draftId, 'Draft')
const findProposal = (id: ID) => mustFind(db.taskProposals, (p) => p.id === id, 'Proposal')

/** Names to show beside a proposal: phases, objectives, operators and astronauts. */
export interface ReviewContext {
  epics: Epic[]
  features: Feature[]
  users: User[]
  agents: Agent[]
}

export async function getReviewContext(draftId: ID): Promise<ReviewContext> {
  const { projectId } = findDraft(draftId)
  return delay({
    epics: db.epics.filter((e) => e.projectId === projectId),
    features: db.features.filter((f) => f.projectId === projectId),
    users: db.users,
    agents: db.agents,
  })
}

// Sub-keys of keys.proposals, so every proposal write refreshes them too.
export const useReviewContext = (draftId: ID) =>
  useQuery({ queryKey: [...keys.proposals(draftId), 'context'], queryFn: () => getReviewContext(draftId) })

/** Proposals that still need the operator. */
export const needsReview = (p: TaskProposal) => p.status === 'needs-review' || p.status === 'revised'
export const isApproved = (p: TaskProposal) => p.status === 'approved' || p.status === 'approved-edited'

// ---- Approve ----

export type ProposalEdits = Partial<Pick<TaskProposal, 'title' | 'description' | 'acceptanceCriteria' | 'estimate' | 'priority'>>

/** Next display key, e.g. PAY-63: one past the highest number in use. */
function nextKey(prefix: string, used: string[]) {
  const nums = used.map((k) => Number(k.slice(prefix.length + 1))).filter(Number.isFinite)
  return `${prefix}-${Math.max(0, ...nums) + 1}`
}

/**
 * Approve a proposal: it becomes a task in the Hangar, under the draft's
 * objective and in the objective's target launch window. With edits, the
 * proposal is marked approved-edited.
 */
export async function approveProposal({ draftId, proposalId, edits }: { draftId: ID; proposalId: ID; edits?: ProposalEdits }): Promise<Task> {
  const draft = findDraft(draftId)
  const p = findProposal(proposalId)
  if (!needsReview(p)) throw new Error('This proposal has already been reviewed.')
  const feature = mustFind(db.features, (f) => f.id === draft.featureId, 'Objective')
  if (edits) {
    if (edits.title !== undefined && !edits.title.trim()) throw new Error('Give the payload a title.')
    if (edits.estimate !== undefined && !(edits.estimate >= 0)) throw new Error('Estimate must be 0 or more.')
    Object.assign(p, edits)
  }

  const backlog = db.tasks.filter((t) => t.projectId === draft.projectId && t.column === 'backlog')
  const task: Task = {
    id: newId('t'),
    key: nextKey('PAY', db.tasks.map((t) => t.key)),
    projectId: draft.projectId,
    featureId: feature.id,
    epicId: p.epicId,
    title: p.title.trim(),
    description: p.description,
    assignee: p.suggestedAssignee,
    estimate: p.estimate,
    priority: p.priority,
    sprintId: feature.sprintId,
    tags: [],
    acceptanceCriteria: p.acceptanceCriteria.map((text) => ({ id: newId('c'), text, met: false })),
    autonomyOverride: null,
    column: 'backlog',
    position: Math.max(0, ...backlog.map((t) => t.position)) + 1,
    status: 'queued',
    blockedReason: null,
    subtasks: { done: 0, total: 0 },
    prNumber: null,
    version: 1,
  }
  db.tasks.push(task)
  db.activityEvents.push({
    id: newId('ev'),
    projectId: draft.projectId,
    at: new Date().toISOString(),
    actor: { kind: 'user', id: signedInUserId },
    verb: 'created',
    taskId: task.id,
    detail: `from the ${feature.name} plan`,
    needsAction: false,
  })
  p.status = edits ? 'approved-edited' : 'approved'
  p.taskId = task.id
  return delay(task)
}

// ---- Deny ----

export interface ProposalFeedback {
  outcome: FeedbackOutcome
  reasons: string[]
  text: string
}

/** Deny a proposal. Revise sends it back to the planning agent; discard drops it. */
export async function denyProposal({ proposalId, feedback }: { proposalId: ID; feedback: ProposalFeedback }): Promise<TaskProposal> {
  const p = findProposal(proposalId)
  if (!feedback.text.trim()) throw new Error('Feedback is required.')
  db.feedback.push({
    id: newId('fb'),
    targetType: 'proposal',
    targetId: proposalId,
    ...feedback,
    sectionKey: null,
    at: new Date().toISOString(),
  })
  p.status = feedback.outcome === 'revise' ? 'sent-back' : 'dropped'
  return delay(p)
}

/**
 * Stand-in for the planning agent coming back with a revision: the proposal
 * returns to the queue, in place, with a note on what changed.
 */
export async function reviseProposal({ proposalId, feedbackText }: { proposalId: ID; feedbackText: string }): Promise<TaskProposal | null> {
  const p = db.taskProposals.find((x) => x.id === proposalId)
  // Only a proposal still waiting on the agent changes.
  if (!p || p.status !== 'sent-back') return delay(null)
  p.status = 'revised'
  p.agentNote = `Revised after your feedback (“${feedbackText}”). I narrowed the scope and rewrote the acceptance criteria to match.`
  return delay(p)
}

/** How long the simulated planning agent takes to revise. */
export const REVISE_DELAY_MS = 1500

/** Deny, and after a revise, simulate the agent's revision a moment later. */
export function useDenyProposal(draftId: ID) {
  const client = useQueryClient()
  const refresh = () => client.invalidateQueries({ queryKey: keys.proposals(draftId) })
  return useMutation({
    mutationFn: denyProposal,
    onSuccess: (_p, { proposalId, feedback }) => {
      if (feedback.outcome === 'revise') {
        setTimeout(() => void reviseProposal({ proposalId, feedbackText: feedback.text.trim() }).then(refresh), REVISE_DELAY_MS)
      }
      return refresh()
    },
  })
}

export const useApproveProposal = (draftId: ID, projectId: ID) =>
  useMockMutation(approveProposal, [keys.proposals(draftId), keys.board(projectId), keys.activity(projectId), ['feature', projectId]])

// ---- Done (step 4) ----

export interface PlanSummary {
  approved: { proposalId: ID; taskId: ID; key: string; title: string; column: ColumnKey }[]
  sentBack: number
  dropped: number
  waiting: number
  total: number
}

export async function getPlanSummary(draftId: ID): Promise<PlanSummary> {
  const proposals = db.taskProposals.filter((p) => p.draftId === draftId).sort((a, b) => a.order - b.order)
  const approved = proposals.flatMap((p) => {
    const task = isApproved(p) ? db.tasks.find((t) => t.id === p.taskId) : undefined
    return task ? [{ proposalId: p.id, taskId: task.id, key: task.key, title: task.title, column: task.column }] : []
  })
  return delay({
    approved,
    sentBack: proposals.filter((p) => p.status === 'sent-back').length,
    dropped: proposals.filter((p) => p.status === 'dropped').length,
    waiting: proposals.filter(needsReview).length,
    total: proposals.length,
  })
}

export const usePlanSummary = (draftId: ID) =>
  useQuery({ queryKey: [...keys.proposals(draftId), 'summary'], queryFn: () => getPlanSummary(draftId) })

