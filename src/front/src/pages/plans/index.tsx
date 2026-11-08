import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { Button, Field, TextInput, Textarea } from '../../components/controls'
import { Dialog } from '../../components/Dialog'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { Tabs } from '../../components/Tabs'
import { usePlan, usePlanAction, usePlans, useStartPlan, type Plan } from '../../data/views/plans'

const statusText: Record<Plan['status'], string> = {
  briefing: 'Interview',
  needs_review: 'Doc to review',
  approved: 'Tasks',
  done: 'Done',
  discarded: 'Discarded',
}

const pendingText: Record<NonNullable<Plan['pending']>, string> = {
  ask: 'The interviewer is thinking…',
  draft: 'The interviewer is writing the doc…',
  generate: 'The task agent is proposing tasks…',
  revise: 'The task agent is revising a task…',
}

/** Planning opens over the board, and closing it goes back to the board. */
function PlanDialog({ children }: { children: ReactNode }) {
  const { projectId = '' } = useParams()
  const navigate = useNavigate()
  return (
    <Dialog label="Plan a feature" wide onClose={() => navigate(`/projects/${projectId}`)}>
      <div className="flex flex-col gap-4 p-6 pr-14">{children}</div>
    </Dialog>
  )
}

/** Start a plan, and the project's plans, newest first. */
export function PlansPage() {
  const { projectId = '' } = useParams()
  const navigate = useNavigate()
  const plans = usePlans(projectId)
  const start = useStartPlan(projectId)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')

  const submit = (e: FormEvent) => {
    e.preventDefault()
    start.mutate({ title, description }, { onSuccess: (p) => navigate(`/projects/${projectId}/plans/${p.id}`) })
  }

  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') ?? 'new'
  const list = (keep: (p: Plan) => boolean) => {
    const shown = plans.data?.filter(keep) ?? []
    return (
      <div className="flex flex-col gap-2 pt-4">
        {plans.error && <p className="m-0 text-sm text-critical">{plans.error.message}</p>}
        {plans.data && shown.length === 0 && <p className="m-0 text-[13px] text-text-muted">No plans here yet.</p>}
        <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
          {shown.map((p) => (
            <li key={p.id}>
              <Link
                to={`/projects/${projectId}/plans/${p.id}`}
                className="flex items-center justify-between gap-3 rounded-lg border border-line-strong bg-surface-sunken px-3 py-2 text-sm text-text no-underline hover:border-ion"
              >
                {p.title}
                <span className="text-xs text-text-muted">{p.pending ? pendingText[p.pending] : statusText[p.status]}</span>
              </Link>
            </li>
          ))}
        </ul>
      </div>
    )
  }
  const active = (p: Plan) => p.status !== 'done' && p.status !== 'discarded'
  const activeCount = plans.data?.filter(active).length ?? 0

  return (
    <PlanDialog>
      <h1 className="m-0 text-xl">Plan a feature</h1>
      <Tabs
        label="Plans"
        value={tab}
        onChange={(id) => setParams(id === 'new' ? {} : { tab: id }, { replace: true })}
        items={[
          {
            id: 'new',
            label: 'New plan',
            content: (
              <form onSubmit={submit} className="flex flex-col gap-3 pt-4">
                <Field id="plan-title" label="Feature">
                  <TextInput id="plan-title" value={title} onChange={(e) => setTitle(e.target.value)} required maxLength={256} />
                </Field>
                <Field id="plan-description" label="Describe it">
                  <Textarea id="plan-description" value={description} onChange={(e) => setDescription(e.target.value)} required />
                </Field>
                {start.error && (
                  <p role="alert" className="m-0 text-sm text-critical">
                    {start.error.message} {/* planning_not_set_up: the interviewer has no model yet */}
                    <Link to="/settings">Settings</Link>
                  </p>
                )}
                <Button type="submit" variant="primary" disabled={start.isPending} className="self-start">
                  Start interview
                </Button>
              </form>
            ),
          },
          { id: 'active', label: `In progress${activeCount ? ` (${activeCount})` : ''}`, content: list(active) },
          { id: 'done', label: 'Done', content: list((p) => !active(p)) },
        ]}
      />
    </PlanDialog>
  )
}

/** One plan: answer the interviewer, approve the doc, then approve the proposed tasks. */
export function PlanPage() {
  const { projectId = '', planId = '' } = useParams()
  const { data: plan, error, isPending } = usePlan(projectId, planId)
  const act = usePlanAction(projectId, planId)
  const [answer, setAnswer] = useState('')
  const [rejecting, setRejecting] = useState<number | 'all' | null>(null)
  const [feedback, setFeedback] = useState('')

  if (isPending || error)
    return <PlanDialog>{isPending ? <StatusMessage title="Loading plan…" /> : <StatusMessage title="Something went wrong" body={error.message} />}</PlanDialog>

  const send = (path: string, body?: unknown) => act.mutate({ path, body }, { onSuccess: () => setAnswer('') })
  const busy = act.isPending || plan.pending !== null
  const messages = plan.messages ?? []
  const last = messages[messages.length - 1]
  const question = plan.status === 'briefing' && !plan.pending && last?.role === 'agent' ? last : null
  const toReview = (plan.proposals ?? []).filter((p) => p.status === 'needs_review')
  const deniable = (plan.proposals ?? []).filter((p) => p.status === 'needs_review' || p.status === 'sent_back')

  // One by one: each answers the whole plan, and a failure stops the rest with its message shown.
  const deny = async (ids: number[], outcome: 'revise' | 'drop') => {
    for (const id of ids) await act.mutateAsync({ path: `proposals/${id}/deny`, body: { feedback: feedback.trim(), outcome } })
    setRejecting(null)
    setFeedback('')
  }
  const rejectBox = (what: string, ids: number[]) => (
    <div className="mt-2 flex flex-wrap items-center gap-2">
      <TextInput
        aria-label={`Why reject ${what}`}
        placeholder="Why? The agent revises from this."
        value={feedback}
        onChange={(e) => setFeedback(e.target.value)}
        className="min-h-9 flex-1 text-sm"
      />
      {(['revise', 'drop'] as const).map((outcome) => (
        <Button
          key={outcome}
          variant={outcome === 'revise' ? 'primary' : 'secondary'}
          className="min-h-9 px-3 text-sm"
          disabled={busy || !feedback.trim()}
          onClick={() => deny(ids, outcome).catch(() => {})}
        >
          {outcome === 'revise' ? 'Revise' : 'Drop'}
        </Button>
      ))}
    </div>
  )

  return (
    <PlanDialog>
      <Link to={`/projects/${projectId}/plans?tab=active`} className="text-sm">
        ← Plans in progress
      </Link>
      <h1 className="m-0 text-xl">
        {plan.title} <span className="text-base text-text-muted">{statusText[plan.status]}</span>
      </h1>
      {plan.pending && <p className="m-0 text-ion">{pendingText[plan.pending]}</p>}
      {plan.error && (
        <p role="alert" className="m-0 text-critical">
          {plan.error} <Button onClick={() => send('retry')}>Retry</Button>
        </p>
      )}
      {act.error && (
        <p role="alert" className="m-0 text-critical">
          {act.error.message}
        </p>
      )}

      {plan.status === 'briefing' && (
        <section className="flex flex-col gap-3">
          <h2 className="eyebrow m-0">Interview</h2>
          {messages.map((m) => (
            <p key={m.id} className={m.role === 'agent' ? 'm-0' : 'm-0 pl-6 text-text-soft'}>
              {m.role === 'agent' ? 'Q: ' : 'A: '}
              {m.skipped ? <em>skipped</em> : m.text}
            </p>
          ))}
          {question && (
            <form
              className="flex flex-col gap-2"
              onSubmit={(e) => {
                e.preventDefault()
                send('answer', { text: answer })
              }}
            >
              <div className="flex flex-wrap gap-2">
                {question.options.map((o) => (
                  <Button key={o} disabled={busy} onClick={() => send('answer', { text: o })}>
                    {o}
                  </Button>
                ))}
              </div>
              <Textarea aria-label="Your answer" value={answer} onChange={(e) => setAnswer(e.target.value)} />
              <div className="flex gap-2">
                <Button type="submit" variant="primary" disabled={busy || !answer.trim()}>
                  Answer
                </Button>
                <Button disabled={busy} onClick={() => send('answer', { skip: true })}>
                  Skip
                </Button>
                <Button disabled={busy} onClick={() => send('draft')}>
                  Write the doc now
                </Button>
              </div>
            </form>
          )}
        </section>
      )}

      {plan.status === 'needs_review' && (
        <Button variant="primary" disabled={busy} onClick={() => send('approve')} className="self-start">
          Approve doc and propose tasks
        </Button>
      )}

      {(plan.proposals ?? []).length > 0 && (
        <section className="flex flex-col gap-2">
          <div className="flex items-center gap-3">
            <h2 className="eyebrow m-0">Proposed tasks</h2>
            {toReview.length > 1 && (
              // Sequential, in position order, so a task's dependencies get their issues first.
              <Button
                disabled={busy}
                className="min-h-9 px-3 text-sm"
                onClick={async () => {
                  for (const p of toReview) await act.mutateAsync({ path: `proposals/${p.id}/approve` })
                }}
              >
                <Icon name="check" size={14} className="text-good" />
                Approve all
              </Button>
            )}
            {deniable.length > 1 && (
              <Button
                disabled={busy}
                aria-expanded={rejecting === 'all'}
                className="min-h-9 px-3 text-sm"
                onClick={() => setRejecting(rejecting === 'all' ? null : 'all')}
              >
                <Icon name="close" size={14} className="text-critical" />
                Reject all
              </Button>
            )}
          </div>
          {rejecting === 'all' && rejectBox('all', deniable.map((p) => p.id))}
          <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
            {plan.proposals!.map((p) => (
              <li key={p.id} className="rounded-lg border border-line-strong bg-surface-sunken px-3 py-2">
                <div className="flex items-start gap-3">
                  <details className="min-w-0 flex-1">
                    <summary className="cursor-pointer text-sm">
                      {p.position}. {p.title} <span className="text-xs text-text-muted">{p.status.replace('_', ' ')}</span>
                    </summary>
                    <p className="mt-1.5 mb-0 text-[13px] whitespace-pre-wrap text-text-soft">{p.description}</p>
                    <ul className="mt-1 mb-0 pl-4 text-xs text-text-muted">
                      {p.tests.map((t, i) => (
                        <li key={i}>
                          {t.kind}: {t.command ? <code>{t.command}</code> : t.name}
                        </li>
                      ))}
                    </ul>
                  </details>
                  {p.taskId && (
                    <Link to={`/projects/${projectId}/tasks/${p.taskId}`} className="text-xs whitespace-nowrap">
                      Open task
                    </Link>
                  )}
                  {(p.status === 'needs_review' || p.status === 'sent_back') && (
                    <div role="group" aria-label={`Decide on ${p.title}`} className="inline-flex flex-none overflow-hidden rounded-lg border border-line-strong bg-surface">
                      <button
                        type="button"
                        aria-label={`Approve ${p.title}`}
                        title="Approve, add to Backlog"
                        disabled={busy || p.status !== 'needs_review'}
                        onClick={() => send(`proposals/${p.id}/approve`)}
                        className="flex size-8 cursor-pointer items-center justify-center border-0 bg-good/10 text-good hover:bg-good/25 disabled:cursor-not-allowed disabled:opacity-40"
                      >
                        <Icon name="check" size={16} />
                      </button>
                      <button
                        type="button"
                        aria-label={`Reject ${p.title}`}
                        title="Reject"
                        aria-expanded={rejecting === p.id}
                        disabled={busy}
                        onClick={() => setRejecting(rejecting === p.id ? null : p.id)}
                        className="flex size-8 cursor-pointer items-center justify-center border-0 border-l border-line-strong bg-critical/10 text-critical hover:bg-critical/25 disabled:cursor-not-allowed disabled:opacity-40"
                      >
                        <Icon name="close" size={16} />
                      </button>
                    </div>
                  )}
                </div>
                {rejecting === p.id && rejectBox(p.title, [p.id])}
              </li>
            ))}
          </ul>
        </section>
      )}

      <section className="flex flex-col gap-3">
        <h2 className="eyebrow m-0">Doc</h2>
        {plan.doc.map((s) => (
          <div key={s.key}>
            <h3 className="m-0 text-sm">{s.title}</h3>
            {Array.isArray(s.body) ? (
              <ul className="m-0 text-text-soft">
                {s.body.map((b, i) => (
                  <li key={i}>{b}</li>
                ))}
              </ul>
            ) : (
              <p className="m-0 whitespace-pre-wrap text-text-soft">{s.body || '—'}</p>
            )}
          </div>
        ))}
      </section>
    </PlanDialog>
  )
}
