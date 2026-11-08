import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { Button, Field, TextInput, Textarea } from '../../components/controls'
import { Dialog } from '../../components/Dialog'
import { StatusMessage } from '../../components/StatusMessage'
import { Tabs } from '../../components/Tabs'
import { Proposals } from './Proposals'
import { errorCode } from '../../data/errors'
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
        {plans.error && !plans.data && (
          <p role="alert" className="m-0 flex flex-wrap items-center gap-3 text-sm text-critical">
            <span className="min-w-0 wrap-anywhere">{plans.error.message}</span>
            <Button onClick={() => plans.refetch()}>Try again</Button>
          </p>
        )}
        {plans.data && shown.length === 0 && <p className="m-0 text-[13px] text-text-muted">No plans here yet.</p>}
        <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
          {shown.map((p) => (
            <li key={p.id}>
              <Link
                to={`/projects/${projectId}/plans/${p.id}`}
                className="flex items-center justify-between gap-3 rounded-lg border border-line-strong bg-surface-sunken px-3 py-2 text-sm text-text no-underline hover:border-ion"
              >
                <span className="min-w-0 wrap-anywhere">{p.title}</span>
                <span className="flex-none text-xs whitespace-nowrap text-text-muted">{p.pending ? pendingText[p.pending] : statusText[p.status]}</span>
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
                  <p role="alert" className="m-0 text-sm wrap-anywhere text-critical">
                    {start.error.message}
                    {errorCode(start.error) === 'planning_not_set_up' && <> <Link to="/settings">Settings</Link></>}
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
  const { data: plan, error, isPending, refetch } = usePlan(projectId, planId)
  const act = usePlanAction(projectId, planId)
  const [answer, setAnswer] = useState('')

  if (!plan)
    return (
      <PlanDialog>
        {isPending ? (
          <StatusMessage title="Loading plan…" />
        ) : (
          <StatusMessage title="Something went wrong" body={error?.message}>
            <Button onClick={() => refetch()}>Try again</Button>
          </StatusMessage>
        )}
      </PlanDialog>
    )

  const send = (path: string, body?: unknown) => act.mutate({ path, body }, { onSuccess: () => setAnswer('') })
  const busy = act.isPending || plan.pending !== null
  const messages = plan.messages ?? []
  const last = messages[messages.length - 1]
  const question = plan.status === 'briefing' && !plan.pending && last?.role === 'agent' ? last : null

  return (
    <PlanDialog>
      <Link to={`/projects/${projectId}/plans?tab=active`} className="text-sm">
        ← Plans in progress
      </Link>
      <h1 className="m-0 text-xl wrap-anywhere">
        {plan.title} <span className="text-base text-text-muted">{statusText[plan.status]}</span>
      </h1>
      {plan.pending && <p className="m-0 text-ion">{pendingText[plan.pending]}</p>}
      {plan.error && (
        <p role="alert" className="m-0 wrap-anywhere text-critical">
          {plan.error} <Button onClick={() => send('retry')}>Retry</Button>
        </p>
      )}
      {act.error && (
        <p role="alert" className="m-0 wrap-anywhere text-critical">
          {act.error.message}
        </p>
      )}

      {plan.status === 'briefing' && (
        <section className="flex flex-col gap-3">
          <h2 className="eyebrow m-0">Interview</h2>
          {messages.map((m) => (
            <p key={m.id} className={m.role === 'agent' ? 'm-0 wrap-anywhere' : 'm-0 pl-6 wrap-anywhere text-text-soft'}>
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
                  <Button key={o} disabled={busy} onClick={() => send('answer', { text: o })} className="max-w-full wrap-anywhere">
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

      {(plan.proposals ?? []).length > 0 && <Proposals projectId={projectId} plan={plan} act={act} busy={busy} />}

      <section className="flex flex-col gap-3 wrap-anywhere">
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
