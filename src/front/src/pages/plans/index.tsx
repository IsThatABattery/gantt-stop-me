import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { Button, Field, TextInput, Textarea } from '../../components/controls'
import { StatusMessage } from '../../components/StatusMessage'
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

  return (
    <div className="flex max-w-3xl flex-col gap-6 p-8">
      <h1 className="m-0 text-2xl">Plan a feature</h1>
      <form onSubmit={submit} className="flex flex-col gap-4">
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
      <section className="flex flex-col gap-2">
        <h2 className="eyebrow m-0">Plans</h2>
        {plans.error && <p className="m-0 text-sm text-critical">{plans.error.message}</p>}
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {plans.data?.map((p) => (
            <li key={p.id}>
              <Link to={`/projects/${projectId}/plans/${p.id}`}>{p.title}</Link>{' '}
              <span className="text-sm text-text-muted">{statusText[p.status]}</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}

/** One plan: answer the interviewer, approve the doc, then approve the proposed tasks. */
export function PlanPage() {
  const { projectId = '', planId = '' } = useParams()
  const { data: plan, error, isPending } = usePlan(projectId, planId)
  const act = usePlanAction(projectId, planId)
  const [answer, setAnswer] = useState('')

  if (isPending) return <StatusMessage title="Loading plan…" />
  if (error) return <StatusMessage title="Something went wrong" body={error.message} />

  const send = (path: string, body?: unknown) => act.mutate({ path, body }, { onSuccess: () => setAnswer('') })
  const busy = act.isPending || plan.pending !== null
  const messages = plan.messages ?? []
  const last = messages[messages.length - 1]
  const question = plan.status === 'briefing' && !plan.pending && last?.role === 'agent' ? last : null
  const toReview = (plan.proposals ?? []).filter((p) => p.status === 'needs_review')

  return (
    <div className="flex max-w-3xl flex-col gap-6 p-8">
      <Link to={`/projects/${projectId}/plans`} className="text-sm">
        ← Plans
      </Link>
      <h1 className="m-0 text-2xl">
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
        <section className="flex flex-col gap-3">
          <div className="flex items-center gap-3">
            <h2 className="eyebrow m-0">Proposed tasks</h2>
            {toReview.length > 1 && (
              // Sequential, in position order, so a task's dependencies get their issues first.
              <Button
                disabled={busy}
                onClick={async () => {
                  for (const p of toReview) await act.mutateAsync({ path: `proposals/${p.id}/approve` })
                }}
              >
                Approve all
              </Button>
            )}
          </div>
          {plan.proposals!.map((p) => (
            <article key={p.id} className="flex flex-col gap-1 rounded-lg border border-line p-4">
              <h3 className="m-0 text-base">
                {p.position}. {p.title} <span className="text-sm text-text-muted">{p.status.replace('_', ' ')}</span>
              </h3>
              <p className="m-0 whitespace-pre-wrap text-text-soft">{p.description}</p>
              <ul className="m-0 text-sm text-text-soft">
                {p.tests.map((t, i) => (
                  <li key={i}>
                    Test ({t.kind}): {t.name}
                    {t.command && <code> {t.command}</code>}
                  </li>
                ))}
              </ul>
              {p.status === 'needs_review' && (
                <Button variant="primary" disabled={busy} onClick={() => send(`proposals/${p.id}/approve`)} className="self-start">
                  Approve, add to Backlog
                </Button>
              )}
              {p.taskId && (
                <Link to={`/projects/${projectId}/tasks/${p.taskId}`} className="text-sm">
                  Open task
                </Link>
              )}
            </article>
          ))}
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
    </div>
  )
}
