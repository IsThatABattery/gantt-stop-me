import { useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { Chip, type Tone } from '../../../components/Chip'
import { FeedbackForm } from '../../../components/FeedbackForm'
import { StatusMessage } from '../../../components/StatusMessage'
import { Button, Textarea } from '../../../components/controls'
import { buttonClass } from '../../../components/buttonClass'
import { useApproveDoc, useDenyDoc, useDraft, useUpdateSections } from '../../../data/views/plannerDoc'
import type { DraftStatus, FeatureDraft } from '../../../data/types'
import { draftStatusLabels, labels, plannerLabels } from '../../../labels/labels'
import { DocSectionView } from './DocSectionView'
import { fromText, sectionAnchor, toText } from './docText'

const agentWord = labels.agent.toLowerCase()
const featureWord = labels.feature.toLowerCase()

const statusTone: Record<DraftStatus, Tone> = {
  briefing: 'neutral',
  'needs-review': 'warning',
  approved: 'good',
  denied: 'warning',
  discarded: 'neutral',
}

type Mode = 'decide' | 'deny' | 'edit'

/** Step 2 (PlanDoc and PlanDocDeny): review the drafted doc. */
export function DocReviewPage() {
  const { projectId = '', draftId = '' } = useParams()
  const { data: draft } = useDraft(draftId)
  if (!draft) return <StatusMessage title="Loading…" />
  if (draft.status === 'briefing') {
    return (
      <StatusMessage title="No doc yet" body={`The briefing ${agentWord} drafts the doc once the briefing is done.`}>
        <Link to={`/projects/${projectId}/plan/${draftId}/briefing`} className={buttonClass('secondary')}>
          Back to the briefing
        </Link>
      </StatusMessage>
    )
  }
  return <DocReview key={draft.id} draft={draft} />
}

function DocReview({ draft }: { draft: FeatureDraft }) {
  const { projectId = '' } = useParams()
  const navigate = useNavigate()
  const base = `/projects/${projectId}/plan/${draft.id}`
  const [mode, setMode] = useState<Mode>('decide')
  const [edits, setEdits] = useState<Record<string, string>>({})
  const approve = useApproveDoc(draft.id, projectId)
  const deny = useDenyDoc(draft.id)
  const save = useUpdateSections(draft.id)
  const reviewing = draft.status === 'needs-review'
  const answers = draft.answered

  const startEdit = () => {
    setEdits(Object.fromEntries(draft.sections.map((s) => [s.key, toText(s)])))
    setMode('edit')
  }
  const saveEdit = () =>
    save.mutate(
      { draftId: draft.id, sections: draft.sections.map((s) => fromText(s, edits[s.key] ?? toText(s))) },
      { onSuccess: () => setMode('decide') },
    )

  return (
    <div className="flex flex-col gap-6">
      <div>
        <div className="eyebrow">STEP 2 · {plannerLabels.steps[1].toUpperCase()}</div>
        <h1 className="mt-1.5 mb-0 font-heading text-[28px] font-semibold">
          {mode === 'deny' ? `Deny the ${featureWord} doc` : `Review the ${featureWord} doc`}
        </h1>
      </div>

      <div className="flex flex-wrap items-start gap-6">
        <article aria-label={`${labels.feature} doc`} className="flex min-w-0 flex-[999_1_600px] flex-col gap-5 rounded-[14px] border border-line bg-surface p-8">
          <div>
            <div className="flex flex-wrap items-center gap-2.5">
              <Chip tone={statusTone[draft.status]} className="px-2.5">
                {draftStatusLabels[draft.status]}
              </Chip>
              <span className="text-xs text-text-muted">
                Version {draft.version} · drafted by the briefing {agentWord} from {answers} {answers === 1 ? 'answer' : 'answers'}
              </span>
            </div>
            <h2 className="mt-3 mb-0 font-heading text-[30px] font-semibold">{draft.name}</h2>
          </div>

          {mode === 'edit'
            ? draft.sections.map((s) => (
                <div key={s.key} className="flex flex-col gap-2 border-t border-line-soft pt-5">
                  <label htmlFor={`edit-${s.key}`} className="font-heading text-base font-semibold">
                    {s.title}
                  </label>
                  {Array.isArray(s.body) && <span className="text-xs text-text-muted">One item per line</span>}
                  <Textarea
                    id={`edit-${s.key}`}
                    rows={Array.isArray(s.body) ? Math.max(3, s.body.length + 1) : 3}
                    value={edits[s.key] ?? ''}
                    onChange={(e) => setEdits((x) => ({ ...x, [s.key]: e.target.value }))}
                  />
                </div>
              ))
            : draft.sections.map((s) => <DocSectionView key={s.key} section={s} />)}
        </article>

        <aside
          aria-label={mode === 'deny' ? plannerLabels.denyDoc : 'Your decision'}
          className="flex min-w-0 flex-[1_1_380px] flex-col gap-4"
        >
          {mode === 'deny' ? (
            <div className="rounded-[14px] border border-line bg-surface-sunken p-6">
              <FeedbackForm
                heading="Why are you denying this doc?"
                intro={`The briefing ${agentWord} will revise the doc and bring it back for review. Your feedback stays attached to this version.`}
                outcomes={plannerLabels.docOutcomes}
                reasons={[...plannerLabels.docReasons]}
                sections={draft.sections.map((s) => ({ key: s.key, title: s.title }))}
                placeholder="e.g. Success criteria needs a real latency target, and conflicts should never be settled automatically."
                submitting={deny.isPending}
                onCancel={() => setMode('decide')}
                onSubmit={(feedback) =>
                  deny.mutate(
                    { draftId: draft.id, feedback },
                    { onSuccess: () => navigate(feedback.outcome === 'revise' ? `${base}/briefing` : `/projects/${projectId}`) },
                  )
                }
              />
            </div>
          ) : mode === 'edit' ? (
            <Panel title="Editing the doc" tone="border-ion">
              <p className="m-0 leading-normal text-text-muted-strong">Change any section, then save. The doc stays waiting for your review.</p>
              <Button variant="primary" onClick={saveEdit} disabled={save.isPending}>
                Save changes
              </Button>
              <Button onClick={() => setMode('decide')}>Cancel</Button>
            </Panel>
          ) : reviewing ? (
            <Panel title="Your decision" tone="border-warning">
              <p className="m-0 leading-normal text-text-muted-strong">
                Approving hands this doc to the planning {agentWord}. It proposes {labels.tasks.toLowerCase()}, and you review each one before
                it reaches the board.
              </p>
              <Button
                variant="primary"
                disabled={approve.isPending}
                onClick={() => approve.mutate({ draftId: draft.id }, { onSuccess: () => navigate(`${base}/tasks`) })}
              >
                {plannerLabels.approveDoc}
              </Button>
              <Button onClick={() => setMode('deny')}>{plannerLabels.denyDoc}</Button>
              <Button onClick={startEdit}>{plannerLabels.editDoc}</Button>
              {approve.error && (
                <p role="alert" className="m-0 text-[13px] text-critical">
                  {approve.error.message}
                </p>
              )}
            </Panel>
          ) : (
            <DonePanel draft={draft} base={base} projectId={projectId} />
          )}

          {draft.comments.length > 0 && (
            <section aria-labelledby="comments-h" className="rounded-[14px] border border-line bg-surface-sunken p-5">
              <h3 id="comments-h" className="mt-0 mb-2.5 font-heading text-[15px] font-semibold">
                Comments ({draft.comments.length})
              </h3>
              <ul className="m-0 flex list-none flex-col gap-3 p-0">
                {draft.comments.map((c) => {
                  const section = draft.sections.find((s) => s.key === c.sectionKey)
                  return (
                    <li key={c.id} className="text-[13px] leading-normal text-text-soft">
                      <span className="text-astronaut-text">{plannerLabels.briefingAgent}</span>
                      {section && (
                        <>
                          {' on '}
                          <a href={`#${sectionAnchor(section.key)}`}>{section.title}</a>
                        </>
                      )}
                      : {c.text}
                    </li>
                  )
                })}
              </ul>
            </section>
          )}
        </aside>
      </div>
    </div>
  )
}

function Panel({ title, tone, children }: { title: string; tone: string; children: ReactNode }) {
  return (
    <section className={`flex flex-col gap-3 rounded-[14px] border bg-surface-sunken p-6 ${tone}`}>
      <h2 className="m-0 font-heading text-lg font-semibold">{title}</h2>
      {children}
    </section>
  )
}

/** The decision panel once the doc is approved, sent back or discarded. */
function DonePanel({ draft, base, projectId }: { draft: FeatureDraft; base: string; projectId: string }) {
  if (draft.status === 'approved') {
    return (
      <Panel title="Approved" tone="border-good">
        <p className="m-0 leading-normal text-text-muted-strong">
          The planning {agentWord} is proposing {labels.tasks.toLowerCase()} from this doc.
        </p>
        <Link to={`${base}/tasks`} className={buttonClass('primary')}>
          {plannerLabels.steps[2]}
        </Link>
      </Panel>
    )
  }
  if (draft.status === 'denied') {
    return (
      <Panel title="Sent back" tone="border-warning">
        <p className="m-0 leading-normal text-text-muted-strong">The briefing {agentWord} is revising the doc with your feedback.</p>
        <Link to={`${base}/briefing`} className={buttonClass('secondary')}>
          Back to the briefing
        </Link>
      </Panel>
    )
  }
  return (
    <Panel title="Discarded" tone="border-line">
      <p className="m-0 leading-normal text-text-muted-strong">Nothing was created from this draft.</p>
      <Link to={`/projects/${projectId}`} className={buttonClass('secondary')}>
        Back to the board
      </Link>
    </Panel>
  )
}
