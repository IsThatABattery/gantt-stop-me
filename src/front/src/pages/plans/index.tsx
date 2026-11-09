import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { Chip } from '../../components/Chip'
import { Button, Field, TextInput, Textarea } from '../../components/controls'
import { Dialog } from '../../components/Dialog'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { Tabs } from '../../components/Tabs'
import { InkTab, Tile } from '../../components/Tile'
import { errorCode } from '../../data/errors'
import { usePlan, usePlanAction, usePlans, useStartPlan, type Plan } from '../../data/views/plans'
import { labels } from '../../labels/labels'
import { band, bandMark, bandPrimary } from '../task-drawer/band'
import { Card } from '../task-drawer/Card'
import { pendingText, planLook } from './look'
import { Proposals } from './Proposals'

/** Planning opens over the board, and closing it goes back to the board. */
function PlanDialog({ children }: { children: ReactNode }) {
  const { projectId = '' } = useParams()
  const navigate = useNavigate()
  return (
    <Dialog label={labels.plans} title={labels.plans} wide onClose={() => navigate(`/projects/${projectId}`)}>
      <div className="flex flex-col gap-5 px-6 pt-5 pb-6 max-sm:px-4">{children}</div>
    </Dialog>
  )
}

function Problem({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <div role="alert" className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] leading-normal text-signal-ink">
      <Icon name="alert" size={16} className="flex-none" />
      <span className="min-w-0 flex-1 wrap-anywhere">{children}</span>
      {action}
    </div>
  )
}

/** Start a plan, and the project's plans, newest first. */
export function PlansPage() {
  const { projectId = '' } = useParams()
  const navigate = useNavigate()
  const readOnly = useProjectContext().permission === 'read'
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
  const active = (p: Plan) => p.status !== 'done' && p.status !== 'discarded'
  const activeCount = plans.data?.filter(active).length ?? 0

  const list = (keep: (p: Plan) => boolean, empty: string) => {
    const shown = plans.data?.filter(keep) ?? []
    return (
      <div className="flex flex-col gap-2 pt-4">
        {/* A failed refresh keeps the list that already loaded. */}
        {plans.error && !plans.data && <Problem action={<Button onClick={() => plans.refetch()}>Try again</Button>}>{plans.error.message}</Problem>}
        {plans.isPending && (
          <p role="status" className="m-0 text-[13px] text-ink-2">
            Loading plans…
          </p>
        )}
        {plans.data && shown.length === 0 && (
          <p className="m-0 rounded-[3px] border border-ink/25 px-4 py-6 text-center text-[13px] text-ink-2">{empty}</p>
        )}
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {shown.map((p) => {
            const look = planLook(p)
            return (
              <li key={p.id}>
                <Link
                  to={`/projects/${projectId}/plans/${p.id}`}
                  className="group block rounded-card border border-line bg-paper p-[3px] text-ink no-underline transition-colors duration-150 hover:border-ink hover:text-ink"
                >
                  <span className="flex min-h-12 items-center justify-between gap-3 rounded-[2px] border border-keyline px-3 py-2 transition-colors duration-150 group-hover:border-ink">
                    <span className="flex min-w-0 flex-col">
                      <span className="text-sm font-semibold wrap-anywhere">{p.title}</span>
                      {p.description && <span className="truncate text-xs text-ink-2">{p.description}</span>}
                    </span>
                    <Chip tone={look.tone} icon={look.icon} iconClassName={look.iconClass} className="flex-none">
                      {look.word}
                    </Chip>
                  </span>
                </Link>
              </li>
            )
          })}
        </ul>
      </div>
    )
  }

  return (
    <PlanDialog>
      <p className="m-0 max-w-[68ch] text-sm leading-normal text-ink-2">
        The interviewer asks about a feature and writes a plan doc. Once you approve the doc, the task agent proposes tasks, and only
        the ones you approve reach the {labels.backlog}.
      </p>
      <Tabs
        label="Plans"
        value={tab}
        onChange={(id) => setParams(id === 'new' ? {} : { tab: id }, { replace: true })}
        items={[
          {
            id: 'new',
            label: 'New plan',
            content: readOnly ? (
              <StatusMessage icon="lock" title="You have read access only" body="Planning a feature needs write access to the repository on GitHub." />
            ) : (
              <form onSubmit={submit} className="flex flex-col gap-4 pt-5">
                <Field id="plan-title" label="Feature">
                  <TextInput
                    id="plan-title"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="e.g. CSV export for reports"
                    required
                    maxLength={256}
                  />
                </Field>
                <Field id="plan-description" label="Describe it">
                  <Textarea
                    id="plan-description"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="What should it do, and for whom? The interviewer asks about the rest."
                    required
                  />
                </Field>
                {start.error && (
                  <Problem>
                    {start.error.message}
                    {/* planning_not_set_up: the interviewer has no model yet */}
                    {errorCode(start.error) === 'planning_not_set_up' && (
                      <>
                        {' '}
                        <Link to="/settings">Settings</Link>
                      </>
                    )}
                  </Problem>
                )}
                <div className="flex justify-end border-t border-rule pt-4">
                  <Button type="submit" variant="primary" busy={start.isPending}>
                    {start.isPending ? 'Starting…' : 'Start interview'}
                  </Button>
                </div>
              </form>
            ),
          },
          {
            id: 'active',
            label: (
              <span className="inline-flex items-center gap-2">
                {'In progress '}
                {activeCount > 0 && <span className="min-w-5 rounded-[2px] border border-current/45 px-1 text-center text-xs leading-[18px]">{activeCount}</span>}
              </span>
            ),
            content: list(active, 'No plans in progress. Start one from New plan.'),
          },
          { id: 'done', label: 'Done', content: list((p) => !active(p), 'No finished plans yet.') },
        ]}
      />
    </PlanDialog>
  )
}

/** One plan: answer the interviewer, approve the doc, then approve the proposed tasks. */
export function PlanPage() {
  const { projectId = '', planId = '' } = useParams()
  const readOnly = useProjectContext().permission === 'read'
  const { data: plan, error, isPending, refetch } = usePlan(projectId, planId)
  const act = usePlanAction(projectId, planId)
  const [answer, setAnswer] = useState('')

  // A failed refresh keeps the plan, and the typed answer. Its own Dialog, so the loaded plan's opens afresh and
  // focuses its first link rather than Close.
  if (!plan)
    return (
      <PlanDialog key="loading">
        {isPending ? (
          <StatusMessage title="Loading plan…" />
        ) : (
          <StatusMessage tone="signal" title="Something went wrong" body={error?.message}>
            <Button onClick={() => refetch()}>Try again</Button>
          </StatusMessage>
        )}
      </PlanDialog>
    )

  const send = (path: string, body?: unknown) => act.mutate({ path, body }, { onSuccess: () => setAnswer('') })
  const busy = readOnly || act.isPending || plan.pending !== null
  const messages = plan.messages ?? []
  const last = messages[messages.length - 1]
  const question = plan.status === 'briefing' && !plan.pending && last?.role === 'agent' ? last : null
  const look = planLook(plan)
  const back = plan.status === 'done' || plan.status === 'discarded' ? 'done' : 'active'

  return (
    <PlanDialog>
      <Link
        to={`/projects/${projectId}/plans?tab=${back}`}
        className="-my-2 inline-flex min-h-11 items-center gap-1.5 self-start text-[13px] font-semibold"
      >
        <Icon name="back" size={16} />
        {back === 'done' ? 'Finished plans' : 'Plans in progress'}
      </Link>

      <header>
        <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
          <h1 className="m-0 min-w-0 text-[22px] leading-[1.25] font-bold tracking-[-0.01em] text-balance wrap-anywhere">{plan.title}</h1>
          <Chip tone={look.tone} icon={look.icon} iconClassName={look.iconClass} className="mt-1 flex-none">
            {look.word}
          </Chip>
        </div>
        {plan.description && <p className="m-0 mt-2 max-w-[68ch] text-sm leading-normal wrap-anywhere text-ink-2">{plan.description}</p>}
        <div role="status">
          {plan.pending && (
            <p className="m-0 mt-3 flex items-center gap-2 text-[13px] font-semibold">
              <Icon name="running" size={16} className="flex-none text-accent motion-safe:animate-live" />
              {pendingText[plan.pending]}
            </p>
          )}
        </div>
      </header>

      {plan.error && (
        <Problem
          action={
            <Button onClick={() => send('retry')} disabled={readOnly || act.isPending}>
              Retry
            </Button>
          }
        >
          {plan.error}
        </Problem>
      )}
      {act.error && <Problem>{act.error.message}</Problem>}

      {plan.status === 'needs_review' && (
        <section aria-labelledby="doc-review" className={`flex flex-wrap items-center gap-x-4 gap-y-3 rounded-card px-4 py-3.5 ${band}`}>
          <span className={bandMark} aria-hidden="true">
            <Icon name="person" size={18} />
          </span>
          <div className="min-w-[14rem] flex-1">
            <h2 id="doc-review" className="m-0 text-[15px] leading-tight font-bold">
              The plan doc is ready for you
            </h2>
            <p className="m-0 mt-0.5 text-[13px] leading-normal">Read it below. Approving it has the task agent propose tasks.</p>
          </div>
          <button type="button" className={bandPrimary} disabled={busy} aria-busy={act.isPending || undefined} onClick={() => send('approve')}>
            <Icon name="check" size={16} />
            Approve doc and propose tasks
          </button>
        </section>
      )}

      {plan.status === 'briefing' && (
        <Tile step={0} as="section" aria-labelledby="interview" frameClassName="gap-3" tab={<InkTab id="interview">Interview</InkTab>}>
          {messages.length === 0 && !plan.pending && <p className="m-0 text-[13px] text-ink-2">The interviewer hasn&rsquo;t asked anything yet.</p>}
          {messages.length > 0 && (
            <ol className="m-0 flex list-none flex-col gap-2 p-0">
              {messages.map((m) =>
                m.role === 'agent' ? (
                  <li key={m.id} className="flex items-start gap-2.5 pr-8 max-sm:pr-0">
                    <span className="mt-1 grid size-7 flex-none place-items-center rounded-[2px] bg-ink text-on-ink" aria-hidden="true">
                      <Icon name="agent" size={16} />
                    </span>
                    <Card className="flex-1">
                      <p className="m-0 text-sm leading-normal wrap-anywhere">
                        <span className="sr-only">The interviewer asked: </span>
                        {m.text}
                      </p>
                    </Card>
                  </li>
                ) : (
                  <li key={m.id} className="flex justify-end pl-12 max-sm:pl-6">
                    <p className="m-0 min-w-0 rounded-card bg-block px-3.5 py-2.5 text-sm leading-normal wrap-anywhere">
                      <span className="sr-only">You answered: </span>
                      {m.skipped ? <span className="text-ink-2">Skipped</span> : m.text}
                    </p>
                  </li>
                ),
              )}
            </ol>
          )}
          {question && (
            <form
              className="flex flex-col gap-2.5 border-t border-tile-keyline pt-3"
              onSubmit={(e) => {
                e.preventDefault()
                send('answer', { text: answer })
              }}
            >
              {question.options.length > 0 && (
                <div role="group" aria-label="Suggested answers" className="flex flex-wrap gap-2">
                  {question.options.map((o) => (
                    <Button key={o} disabled={busy} onClick={() => send('answer', { text: o })} className="max-w-full wrap-anywhere">
                      {o}
                    </Button>
                  ))}
                </div>
              )}
              <Textarea aria-label="Your answer" placeholder="Or write your own answer" rows={3} value={answer} onChange={(e) => setAnswer(e.target.value)} />
              <div className="flex flex-wrap gap-2.5">
                <Button type="submit" variant="primary" disabled={busy || !answer.trim()}>
                  Answer
                </Button>
                <Button disabled={busy} onClick={() => send('answer', { skip: true })}>
                  Skip
                </Button>
                <Button variant="ghost" disabled={busy} onClick={() => send('draft')} className="sm:ml-auto">
                  Write the doc now
                </Button>
              </div>
            </form>
          )}
        </Tile>
      )}

      {(plan.proposals ?? []).length > 0 && <Proposals projectId={projectId} plan={plan} act={act} busy={busy} />}

      <Tile step={0.5} as="section" aria-labelledby="plan-doc" frameClassName="gap-2" tab={<InkTab id="plan-doc">Plan doc</InkTab>}>
        {plan.doc.length === 0 ? (
          <p className="m-0 text-[13px] text-ink-2">The interviewer writes the doc once the interview is done.</p>
        ) : (
          <Card frameClassName="gap-0 divide-y divide-keyline py-0 wrap-anywhere">
            {plan.doc.map((s) => (
              <div key={s.key} className="flex flex-col gap-1 py-3">
                <h3 className="m-0 text-sm font-bold">{s.title}</h3>
                {Array.isArray(s.body) ? (
                  <ul className="m-0 flex list-disc flex-col gap-0.5 pl-5 text-sm leading-normal text-ink marker:text-ink-2">
                    {s.body.map((b, i) => (
                      <li key={i}>{b}</li>
                    ))}
                  </ul>
                ) : s.body ? (
                  <p className="m-0 max-w-[68ch] text-sm leading-normal whitespace-pre-wrap text-ink">{s.body}</p>
                ) : (
                  <p className="m-0 text-[13px] text-ink-2">Not written yet.</p>
                )}
              </div>
            ))}
          </Card>
        )}
      </Tile>
    </PlanDialog>
  )
}
