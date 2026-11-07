import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { Chip, type Tone } from '../../../components/Chip'
import { Icon } from '../../../components/Icon'
import { StatusMessage } from '../../../components/StatusMessage'
import { Button, Textarea } from '../../../components/controls'
import { useUser } from '../../../data/core'
import { useSession } from '../../../data/session'
import { useAnswerQuestion, useDraft, useDraftDoc, useSkipQuestion } from '../../../data/views/plannerDoc'
import type { BriefingMessage, DocSection, DocTopicState, FeatureDraft } from '../../../data/types'
import { labels, plannerLabels, topicStateLabels } from '../../../labels/labels'

const agentWord = labels.agent.toLowerCase()

/** Step 1 (PlanBriefing.dc.html): the briefing agent interviews the operator. */
export function BriefingPage() {
  const { draftId = '' } = useParams()
  const { data: draft } = useDraft(draftId)
  if (!draft) return <StatusMessage title="Loading…" />
  return <Briefing draft={draft} />
}

function Briefing({ draft }: { draft: FeatureDraft }) {
  const { projectId = '' } = useParams()
  const base = `/projects/${projectId}/plan/${draft.id}`
  const live = draft.status === 'briefing' || draft.status === 'denied'

  return (
    <div className="flex flex-wrap items-start gap-6">
      <div className="flex min-w-0 flex-[999_1_600px] flex-col gap-5">
        <div>
          <div className="eyebrow">STEP 1 · {plannerLabels.steps[0].toUpperCase()}</div>
          <h1 className="mt-1.5 mb-0 font-heading text-[28px] font-semibold">Tell the {agentWord} about the {labels.feature.toLowerCase()}</h1>
          <p className="mt-2 mb-0 max-w-[640px] leading-normal text-text-muted-strong">
            The briefing {agentWord} asks one question at a time, then drafts an {labels.feature.toLowerCase()} doc for you to approve. Nothing is
            added to the board yet.
          </p>
        </div>

        <StatusNote draft={draft} base={base} />

        <section aria-label="Conversation" className="rounded-[14px] border border-line bg-surface p-6">
          <ol role="log" aria-label="Messages" className="m-0 flex list-none flex-col gap-[18px] p-0">
            {draft.messages.map((m) => (
              <Message key={m.id} message={m} />
            ))}
          </ol>
        </section>

        {live && <AnswerBox draft={draft} base={base} />}
      </div>

      <DraftColumn sections={draft.sections} />
    </div>
  )
}

function StatusNote({ draft, base }: { draft: FeatureDraft; base: string }) {
  const { projectId } = useParams()
  const note = (tone: string, children: ReactNode) => (
    <div role="status" className={`flex flex-wrap items-center gap-x-4 gap-y-2 rounded-card border px-[18px] py-3.5 ${tone}`}>
      {children}
    </div>
  )
  switch (draft.status) {
    case 'denied':
      return note(
        'border-warning bg-warning/6',
        <span>
          <strong className="font-semibold text-warning">Revising.</strong> You sent the doc back. The briefing {agentWord} is revising it with
          your feedback.
        </span>,
      )
    case 'needs-review':
      return note(
        'border-ion bg-ion/8',
        <>
          <span>The doc is drafted and waiting for your review.</span>
          <Link to={`${base}/doc`} className="ml-auto font-medium">
            {plannerLabels.steps[1]}
          </Link>
        </>,
      )
    case 'approved':
      return note(
        'border-good bg-good/6',
        <>
          <span>You approved the doc.</span>
          <Link to={`${base}/tasks`} className="ml-auto font-medium">
            {plannerLabels.steps[2]}
          </Link>
        </>,
      )
    case 'discarded':
      return note(
        'border-line bg-surface',
        <>
          <span>This draft was discarded.</span>
          <Link to={`/projects/${projectId}`} className="ml-auto font-medium">
            Back to the board
          </Link>
        </>,
      )
    default:
      return null
  }
}

function Message({ message: m }: { message: BriefingMessage }) {
  const session = useSession()
  const { data: user } = useUser(session?.userId)
  if (m.from === 'agent') {
    return (
      <li className="flex max-w-[720px] items-start gap-3">
        <span className="inline-flex size-9 flex-none items-center justify-center rounded-[10px] border border-astronaut bg-astronaut/10 text-astronaut">
          <Icon name="helmet" size={20} />
        </span>
        <div className="rounded-card border border-line bg-surface px-4 py-3.5">
          <div className="mb-1 text-xs text-astronaut-text">{plannerLabels.briefingAgent}</div>
          <div className="leading-[1.55]">{m.text}</div>
        </div>
      </li>
    )
  }
  return (
    <li className="flex items-start justify-end gap-3">
      <div className="max-w-[640px] rounded-card border border-ion bg-ion/8 px-4 py-3.5 leading-[1.55]">
        <span className="sr-only">You: </span>
        {m.text}
      </div>
      <span aria-hidden="true" className="inline-flex size-9 flex-none items-center justify-center rounded-full bg-line text-[13px] font-semibold">
        {user?.initials}
      </span>
    </li>
  )
}

function AnswerBox({ draft, base }: { draft: FeatureDraft; base: string }) {
  const navigate = useNavigate()
  const [text, setText] = useState('')
  const answer = useAnswerQuestion(draft.id)
  const skip = useSkipQuestion(draft.id)
  const drafting = useDraftDoc(draft.id)
  const busy = answer.isPending || skip.isPending || drafting.isPending
  const asking = draft.sections.some((s) => s.topicState === 'asking')
  const last = draft.messages[draft.messages.length - 1]
  const options = asking && last?.from === 'agent' ? (last.options ?? []) : []
  const question = Math.min(draft.answered + 1, draft.expectedQuestions)

  const send = (e: FormEvent) => {
    e.preventDefault()
    if (!text.trim()) return
    answer.mutate({ draftId: draft.id, text }, { onSuccess: () => setText('') })
  }

  const draftNow = () => drafting.mutate({ draftId: draft.id }, { onSuccess: () => navigate(`${base}/doc`) })

  return (
    <form onSubmit={send} aria-label={plannerLabels.yourAnswer} className="flex flex-col gap-3.5 rounded-[14px] border border-line bg-surface-sunken p-5">
      {asking && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <span className="text-[13px] text-text-muted">
            Question {question} of about {draft.expectedQuestions}
          </span>
          <div
            role="img"
            aria-label={`${draft.answered} of ${draft.expectedQuestions} questions answered`}
            className="h-1.5 w-[180px] overflow-hidden rounded-[3px] bg-line-soft"
          >
            <div className="h-1.5 bg-astronaut" style={{ width: `${Math.round((draft.answered / draft.expectedQuestions) * 100)}%` }} />
          </div>
        </div>
      )}

      {options.length > 0 && (
        <div role="group" aria-label="Suggested answers" className="flex flex-wrap gap-2">
          {options.map((o) => (
            <button
              key={o}
              type="button"
              aria-pressed={text === o}
              onClick={() => setText(o)}
              className={`min-h-10 cursor-pointer rounded-full border border-astronaut px-3.5 font-[inherit] text-text ${
                text === o ? 'bg-astronaut/25' : 'bg-astronaut/10 hover:bg-astronaut/20'
              }`}
            >
              {o}
            </button>
          ))}
        </div>
      )}

      <label htmlFor="answer" className="sr-only">
        {plannerLabels.yourAnswer}
      </label>
      <Textarea
        id="answer"
        rows={3}
        value={text}
        placeholder={options.length ? 'Type your answer, or pick a quick reply' : 'Type your answer'}
        onChange={(e) => setText(e.target.value)}
      />

      {(answer.error ?? drafting.error) && (
        <p role="alert" className="m-0 text-[13px] text-critical">
          {(answer.error ?? drafting.error)?.message}
        </p>
      )}

      <div className="flex flex-wrap justify-between gap-2.5">
        <div className="flex flex-wrap gap-2.5">
          {asking && (
            <Button onClick={() => skip.mutate({ draftId: draft.id })} disabled={busy}>
              {plannerLabels.skipQuestion}
            </Button>
          )}
          <Button onClick={draftNow} disabled={busy}>
            {plannerLabels.draftNow}
          </Button>
        </div>
        <Button type="submit" variant="primary" disabled={busy || !text.trim()} className="disabled:cursor-not-allowed disabled:opacity-50">
          {plannerLabels.sendAnswer}
        </Button>
      </div>
    </form>
  )
}

const topicLook: Record<DocTopicState, { tone: Tone; text: string }> = {
  covered: { tone: 'good', text: 'text-good' },
  asking: { tone: 'ion', text: 'text-ion' },
  'not-yet': { tone: 'neutral', text: 'text-text-muted' },
}

function DraftColumn({ sections }: { sections: DocSection[] }) {
  const covered = sections.filter((s) => s.topicState === 'covered').length
  return (
    <aside aria-label={`${labels.feature} doc draft`} className="flex min-w-0 flex-[1_1_360px] flex-col rounded-[14px] border border-line bg-surface-sunken p-6">
      <div className="flex items-baseline justify-between">
        <h2 className="m-0 font-heading text-lg font-semibold">{labels.feature} doc (draft)</h2>
        <span className="font-mono text-xs text-text-muted">
          {covered} of {sections.length} topics
        </span>
      </div>
      <p className="mt-2 mb-2 text-[13px] leading-normal text-text-muted-strong">
        Fills in as you answer. Topics left empty become open questions in the doc.
      </p>
      <ul className="m-0 list-none p-0">
        {sections.map((s) => (
          <li key={s.key} className="flex gap-3 border-t border-line-soft py-3">
            <TopicMark state={s.topicState} />
            <div className="min-w-0 flex-1">
              <div className="flex justify-between gap-2">
                <span className="font-medium">{s.title}</span>
                <Chip tone={topicLook[s.topicState].tone}>{topicStateLabels[s.topicState]}</Chip>
              </div>
              {s.topicState === 'covered' && s.body && (
                <div className="mt-1 line-clamp-3 text-[13px] leading-normal text-text-muted-strong">{Array.isArray(s.body) ? s.body.join(' · ') : s.body}</div>
              )}
            </div>
          </li>
        ))}
      </ul>
    </aside>
  )
}

function TopicMark({ state }: { state: DocTopicState }) {
  if (state === 'covered')
    return (
      <span aria-hidden="true" className="inline-flex size-[22px] flex-none items-center justify-center rounded-full bg-good/15 text-good">
        <Icon name="check" size={14} strokeWidth={2.25} />
      </span>
    )
  if (state === 'asking') return <span aria-hidden="true" className="size-[22px] flex-none rounded-full border-2 border-ion" />
  return <span aria-hidden="true" className="size-[22px] flex-none rounded-full border border-dashed border-line-strong" />
}

