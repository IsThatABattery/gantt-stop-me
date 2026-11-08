import { useState, type ReactNode } from 'react'
import { Link, useParams } from 'react-router'
import { Chip } from '../../../components/Chip'
import { FeedbackForm } from '../../../components/FeedbackForm'
import { Button, Field, Select, Textarea, TextInput } from '../../../components/controls'
import { useDraft } from '../../../data/views/plannerDoc'
import {
  needsReview,
  useApproveProposal,
  useDenyProposal,
  type PlanSummary,
  type ProposalEdits,
  type ReviewContext,
} from '../../../data/views/plannerTasks'
import { PRIORITIES, type Priority, type TaskProposal } from '../../../data/types'
import { labels, plannerLabels, priorityLabels, proposalStatusLabels } from '../../../labels/labels'
import { assigneeText, sourceText, statusTone } from './proposalText'

const agentWord = labels.agent.toLowerCase()

interface ProposalDetailProps {
  proposal: TaskProposal
  proposals: TaskProposal[]
  context: ReviewContext | undefined
  summary: PlanSummary | undefined
  onSelect: (id: string) => void
}

type Mode = 'view' | 'edit' | 'deny'

/** The selected proposal, with Approve, Edit before approving and Deny. */
export function ProposalDetail(props: ProposalDetailProps) {
  // Remount per proposal, so edit and deny state never carries over.
  return <Detail key={props.proposal.id} {...props} />
}

interface EditState {
  title: string
  description: string
  criteria: string
  estimate: string
  priority: Priority
}

const toEdit = (p: TaskProposal): EditState => ({
  title: p.title,
  description: p.description,
  criteria: p.acceptanceCriteria.join('\n'),
  estimate: String(p.estimate),
  priority: p.priority,
})

/** Only the fields the operator changed, or undefined when nothing changed. */
function diff(p: TaskProposal, e: EditState): ProposalEdits | undefined {
  const next: ProposalEdits = {}
  const criteria = e.criteria
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
  if (e.title.trim() !== p.title) next.title = e.title.trim()
  if (e.description.trim() !== p.description) next.description = e.description.trim()
  if (criteria.join('\n') !== p.acceptanceCriteria.join('\n')) next.acceptanceCriteria = criteria
  if (Number(e.estimate) !== p.estimate) next.estimate = e.estimate.trim() === '' ? NaN : Number(e.estimate)
  if (e.priority !== p.priority) next.priority = e.priority
  return Object.keys(next).length ? next : undefined
}

function Detail({ proposal: p, proposals, context, summary, onSelect }: ProposalDetailProps) {
  const { projectId = '', draftId = '' } = useParams()
  const { data: draft } = useDraft(draftId)
  const approve = useApproveProposal(draftId, projectId)
  const deny = useDenyProposal(draftId)
  const [mode, setMode] = useState<Mode>('view')
  const [edit, setEdit] = useState<EditState>(() => toEdit(p))
  const open = needsReview(p)
  const feature = context?.features.find((f) => f.id === draft?.featureId)
  const epic = context?.epics.find((e) => e.id === p.epicId)
  const quote = sourceText(p.source, draft?.sections)
  const task = summary?.approved.find((a) => a.proposalId === p.id)

  const doApprove = () =>
    approve.mutate({ draftId, proposalId: p.id, edits: mode === 'edit' ? diff(p, edit) : undefined }, { onSuccess: () => setMode('view') })

  if (mode === 'deny') {
    return (
      <article aria-label={`Selected ${labels.task.toLowerCase()}`} className={card}>
        <FeedbackForm
          heading={`Why are you denying “${p.title}”?`}
          intro={`The planning ${agentWord} revises or drops it. Your feedback stays attached to this proposal.`}
          outcomes={plannerLabels.taskOutcomes}
          reasons={[...plannerLabels.taskReasons]}
          placeholder="e.g. Split this in two: conflict detection first, the chooser UI second."
          submitting={deny.isPending}
          onCancel={() => setMode('view')}
          onSubmit={({ outcome, reasons, text }) =>
            deny.mutate({ proposalId: p.id, feedback: { outcome, reasons, text } }, { onSuccess: () => setMode('view') })
          }
        />
      </article>
    )
  }

  const set = <K extends keyof EditState>(key: K, value: EditState[K]) => setEdit((e) => ({ ...e, [key]: value }))

  return (
    <article aria-label={`Selected ${labels.task.toLowerCase()}`} className={card}>
      <div className="flex flex-wrap items-center gap-3">
        <Chip tone={statusTone[p.status]} className="px-3">
          {proposalStatusLabels[p.status]}
        </Chip>
        <span className="font-mono text-xs text-text-muted">
          Proposal {p.order} of {proposals.length} · proposed by the planning {agentWord}
        </span>
      </div>

      {mode === 'edit' ? (
        <div className="flex flex-col gap-4">
          <Field id="pe-title" label="Title">
            <TextInput id="pe-title" value={edit.title} onChange={(e) => set('title', e.target.value)} />
          </Field>
          <Field id="pe-desc" label="Description">
            <Textarea id="pe-desc" rows={3} value={edit.description} onChange={(e) => set('description', e.target.value)} />
          </Field>
          <Field id="pe-criteria" label="Acceptance criteria (one per line)">
            <Textarea id="pe-criteria" rows={4} value={edit.criteria} onChange={(e) => set('criteria', e.target.value)} />
          </Field>
          <div className="grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-4">
            <Field id="pe-estimate" label="Estimate (points)">
              <TextInput id="pe-estimate" type="number" min={0} value={edit.estimate} onChange={(e) => set('estimate', e.target.value)} />
            </Field>
            <Field id="pe-priority" label="Priority">
              <Select id="pe-priority" value={edit.priority} onChange={(e) => set('priority', e.target.value as Priority)}>
                {PRIORITIES.map((x) => (
                  <option key={x} value={x}>
                    {priorityLabels[x]}
                  </option>
                ))}
              </Select>
            </Field>
          </div>
        </div>
      ) : (
        <>
          <h2 className="m-0 font-heading text-2xl font-semibold">{p.title}</h2>
          {p.description && <p className="m-0 leading-[1.65] text-text-soft">{p.description}</p>}
          {p.acceptanceCriteria.length > 0 && (
            <div>
              <h3 className="mt-0 mb-2 font-heading text-[15px] font-semibold">Acceptance criteria</h3>
              <ul className="m-0 list-disc pl-5 leading-[1.7] text-text-soft">
                {p.acceptanceCriteria.map((c) => (
                  <li key={c}>{c}</li>
                ))}
              </ul>
            </div>
          )}
        </>
      )}

      <dl className="m-0 grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-4">
        <Fact term={labels.feature}>{feature?.name ?? draft?.name ?? '—'}</Fact>
        <Fact term={labels.epic}>{epic?.name ?? '—'}</Fact>
        {mode !== 'edit' && <Fact term="Estimate">{p.estimate} pt</Fact>}
        {mode !== 'edit' && <Fact term="Priority">{priorityLabels[p.priority]}</Fact>}
        <Fact term="Suggested assignee">{assigneeText(p.suggestedAssignee, context)}</Fact>
        <Fact term="Depends on">
          {p.dependsOn.length === 0
            ? 'Nothing'
            : p.dependsOn.map((id) => {
                const dep = proposals.find((x) => x.id === id)
                return dep ? (
                  <button
                    key={id}
                    type="button"
                    onClick={() => onSelect(id)}
                    className="cursor-pointer border-0 bg-transparent p-0 text-left font-[inherit] text-ion hover:text-ion-hover"
                  >
                    {dep.title}
                  </button>
                ) : null
              })}
        </Fact>
      </dl>

      {(p.agentNote || p.source) && (
        <div className="rounded-control border border-astronaut bg-astronaut/8 px-4 py-4 leading-normal">
          <span className="text-astronaut-text">{plannerLabels.planningAgent}</span>
          {p.source && <> · Source: doc section {p.source}.</>} {p.agentNote}
          {quote && <blockquote className="mx-0 mt-3 mb-0 border-l-2 border-astronaut pl-3 text-[13px] text-text-muted-strong">{quote}</blockquote>}
        </div>
      )}

      {(approve.error ?? deny.error) && (
        <p role="alert" className="m-0 text-[13px] text-critical">
          {(approve.error ?? deny.error)?.message}
        </p>
      )}

      {open ? (
        <>
          <div className="flex flex-wrap gap-3">
            <Button variant="primary" onClick={doApprove} disabled={approve.isPending}>
              {plannerLabels.approveTask}
            </Button>
            {mode === 'edit' ? (
              <Button
                onClick={() => {
                  setEdit(toEdit(p))
                  setMode('view')
                }}
              >
                Cancel editing
              </Button>
            ) : (
              <>
                <Button onClick={() => setMode('edit')}>{plannerLabels.editTask}</Button>
                <Button onClick={() => setMode('deny')}>{plannerLabels.denyTask}</Button>
              </>
            )}
          </div>
          <div className="text-xs text-text-muted">
            Approved {labels.tasks.toLowerCase()} land in the {labels.backlog}, tagged with this {labels.feature.toLowerCase()}.
          </div>
        </>
      ) : (
        <ReviewedNote proposal={p} task={task} projectId={projectId} />
      )}
    </article>
  )
}

const card = 'flex min-w-0 flex-[999_1_520px] flex-col gap-5 rounded-card border border-line bg-surface p-7'

function Fact({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div>
      <dt className="mb-1 text-xs text-text-muted">{term}</dt>
      <dd className="m-0 flex flex-col items-start gap-1 leading-[1.4]">{children}</dd>
    </div>
  )
}

function ReviewedNote({ proposal: p, task, projectId }: { proposal: TaskProposal; task?: PlanSummary['approved'][number]; projectId: string }) {
  const text =
    p.status === 'sent-back'
      ? `Sent back. The planning ${agentWord} is revising it with your feedback.`
      : p.status === 'dropped'
        ? 'Dropped from the plan. Nothing was added to the board.'
        : null
  if (text) return <p className="m-0 text-[13px] text-text-muted-strong">{text}</p>
  return (
    <p className="m-0 text-[13px] text-text-muted-strong">
      In the {labels.backlog}
      {task && (
        <>
          {' as '}
          <Link to={`/projects/${projectId}/tasks/${task.taskId}`} className="font-mono">
            {task.key}
          </Link>
        </>
      )}
      .
    </p>
  )
}
