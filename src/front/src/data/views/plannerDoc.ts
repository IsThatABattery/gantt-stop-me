// Planner steps 1–2: briefing and doc review (feature/frontend-planner-doc).

import { useQuery } from '@tanstack/react-query'
import { keys } from '../keys'
import { briefingScript, doneMessage, fallbackQuestion, notedMessage, revisingMessage } from '../mock/plannerDoc'
import { db, delay, mustFind, newId, NotFoundError, signedInUserId } from '../mock/store'
import { useApiMutation } from '../api/client'
import type { DocSection, Feature, FeatureDraft, FeedbackOutcome, ID } from '../types'

export function getDraft(draftId: ID): Promise<FeatureDraft> {
  const d = db.featureDrafts.find((x) => x.id === draftId)
  return d ? delay(d) : Promise.reject(new NotFoundError('Draft not found'))
}

export const useDraft = (draftId: ID) => useQuery({ queryKey: keys.draft(draftId), queryFn: () => getDraft(draftId) })

// ---- Briefing (step 1) ----

const OPEN_QUESTIONS = 'open-questions'

const findDraft = (draftId: ID) => mustFind(db.featureDrafts, (d) => d.id === draftId, 'Draft')

/** Ask about the next untouched topic after `from`, or say the interview is done. */
function askNext(d: FeatureDraft, from: number) {
  const next = d.sections.findIndex((s, i) => i > from && s.topicState === 'not-yet' && s.key !== OPEN_QUESTIONS)
  if (next === -1) {
    d.messages.push({ id: newId('bm'), from: 'agent', text: doneMessage })
    return
  }
  const section = d.sections[next]
  section.topicState = 'asking'
  const script = briefingScript[section.key]
  d.messages.push({
    id: newId('bm'),
    from: 'agent',
    text: script?.question ?? fallbackQuestion(section.title),
    ...(script?.options && { options: script.options }),
  })
}

/** Send an answer: the current topic becomes covered and the next one is asked. */
export async function answerQuestion({ draftId, text }: { draftId: ID; text: string }): Promise<FeatureDraft> {
  const answer = text.trim()
  if (!answer) throw new Error('Write an answer first.')
  const d = findDraft(draftId)
  d.messages.push({ id: newId('bm'), from: 'user', text: answer })
  const current = d.sections.findIndex((s) => s.topicState === 'asking')
  if (current === -1) {
    d.messages.push({ id: newId('bm'), from: 'agent', text: notedMessage })
  } else {
    Object.assign(d.sections[current], { topicState: 'covered', body: answer })
    d.answered += 1
    askNext(d, current)
  }
  return delay(d)
}

/** Skip the current question: its topic stays empty and the next one is asked. */
export async function skipQuestion({ draftId }: { draftId: ID }): Promise<FeatureDraft> {
  const d = findDraft(draftId)
  const current = d.sections.findIndex((s) => s.topicState === 'asking')
  if (current === -1) return delay(d)
  d.sections[current].topicState = 'not-yet'
  d.messages.push({ id: newId('bm'), from: 'user', text: 'Skip this one for now.' })
  askNext(d, current)
  return delay(d)
}

const asList = (body: string | string[] | undefined) => (Array.isArray(body) ? body : body ? [body] : [])

/**
 * Draft the doc from what is covered. Uncovered topics become items under
 * Open questions, and the doc waits for review.
 */
export async function draftDoc({ draftId }: { draftId: ID }): Promise<FeatureDraft> {
  const d = findDraft(draftId)
  const topics = d.sections.filter((s) => s.key !== OPEN_QUESTIONS)
  const missing = topics.filter((s) => s.topicState !== 'covered')
  const open = [
    ...asList(d.sections.find((s) => s.key === OPEN_QUESTIONS)?.body),
    ...missing.map((s) => `Not discussed yet: ${s.title}`),
  ]
  d.sections = [
    ...topics.filter((s) => s.topicState === 'covered'),
    ...(open.length ? [{ key: OPEN_QUESTIONS, title: 'Open questions', topicState: 'covered' as const, body: open }] : []),
  ]
  d.status = 'needs-review'
  d.version = Math.max(1, d.version + 1)
  return delay(d)
}

// ---- Doc review (step 2) ----

/** The operator's own edits to the doc. */
export async function updateSections({ draftId, sections }: { draftId: ID; sections: DocSection[] }): Promise<FeatureDraft> {
  const d = findDraft(draftId)
  d.sections = sections
  return delay(d)
}

/** Next display key, e.g. OBJ-9: one past the highest number in use. */
function nextKey(prefix: string, used: string[]) {
  const nums = used.map((k) => Number(k.slice(prefix.length + 1))).filter(Number.isFinite)
  return `${prefix}-${Math.max(0, ...nums) + 1}`
}

/** Approve the doc: it becomes an objective and planning moves to payloads. */
export async function approveDoc({ draftId }: { draftId: ID }): Promise<FeatureDraft> {
  const d = findDraft(draftId)
  if (!d.featureId) {
    const section = (key: string) => d.sections.find((s) => s.key === key)?.body
    const summary = section('summary') ?? section('problem')
    const feature: Feature = {
      id: newId('f'),
      key: nextKey('OBJ', db.features.map((f) => f.key)),
      projectId: d.projectId,
      epicId: db.epics.find((e) => e.projectId === d.projectId)?.id ?? '',
      name: d.name,
      description: asList(summary).join(' '),
      ownerId: signedInUserId,
      sprintId: null,
      tags: [],
      successCriteria: asList(section('success')).map((text, i) => ({ id: newId(`c${i + 1}`), text, met: false })),
      defaultAutonomy: null,
      status: 'planned',
    }
    db.features.push(feature)
    d.featureId = feature.id
  }
  d.status = 'approved'
  return delay(d)
}

export interface DocFeedback {
  outcome: FeedbackOutcome
  reasons: string[]
  text: string
  sectionKey: string | null
}

/** Deny the doc. Revise sends it back to the briefing; discard archives it. */
export async function denyDoc({ draftId, feedback }: { draftId: ID; feedback: DocFeedback }): Promise<FeatureDraft> {
  const d = findDraft(draftId)
  if (!feedback.text.trim()) throw new Error('Feedback is required.')
  db.feedback.push({ id: newId('fb'), targetType: 'doc', targetId: draftId, ...feedback, at: new Date().toISOString() })
  if (feedback.outcome === 'revise') {
    d.status = 'denied'
    const where = d.sections.find((s) => s.key === feedback.sectionKey)?.title ?? 'the whole doc'
    d.messages.push({ id: newId('bm'), from: 'agent', text: revisingMessage(where, feedback.text.trim()) })
  } else {
    d.status = 'discarded'
  }
  return delay(d)
}

// ---- Hooks ----

const draftKeys = (draftId: ID) => [keys.draft(draftId)]

export const useAnswerQuestion = (draftId: ID) => useApiMutation(answerQuestion, draftKeys(draftId))
export const useSkipQuestion = (draftId: ID) => useApiMutation(skipQuestion, draftKeys(draftId))
export const useDraftDoc = (draftId: ID) => useApiMutation(draftDoc, draftKeys(draftId))
export const useUpdateSections = (draftId: ID) => useApiMutation(updateSections, draftKeys(draftId))
export const useDenyDoc = (draftId: ID) => useApiMutation(denyDoc, draftKeys(draftId))
export const useApproveDoc = (draftId: ID, projectId: ID) =>
  useApiMutation(approveDoc, [keys.draft(draftId), keys.board(projectId), keys.formOptions(projectId), keys.proposals(draftId)])
