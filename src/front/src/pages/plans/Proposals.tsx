import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { Button, Select, TextInput } from '../../components/controls'
import { Icon } from '../../components/Icon'
import type { Plan, Proposal, usePlanAction } from '../../data/views/plans'

type Choice = { verdict: 'approve' } | { verdict: 'reject'; reason: string; outcome: 'revise' | 'drop' }

const decidable = (p: Proposal) => p.status === 'needs_review' || p.status === 'sent_back'
// A task sent back is with the agent until it returns it, so it can only be dropped or sent back again.
const approvable = (p: Proposal) => p.status === 'needs_review'
const ready = (p: Proposal, c?: Choice) => !!c && (c.verdict === 'approve' ? approvable(p) : !!c.reason.trim())

/**
 * The task agent's proposed tasks. ✓ and ✗ only choose; Confirm sends every choice at once, and a plan with nothing
 * left to decide is done.
 */
export function Proposals({ projectId, plan, act, busy }: { projectId: string; plan: Plan; act: ReturnType<typeof usePlanAction>; busy: boolean }) {
  const navigate = useNavigate()
  const [choices, setChoices] = useState<Record<number, Choice>>({})
  const proposals = plan.proposals ?? []
  const open = proposals.filter(decidable)
  const choose = (id: number, c?: Choice) =>
    setChoices((all) => {
      const next = { ...all }
      if (c) next[id] = c
      else delete next[id]
      return next
    })
  const reject = (id: number): Choice => ({ verdict: 'reject', reason: '', outcome: 'drop', ...(choices[id]?.verdict === 'reject' ? choices[id] : {}) })
  const rejected = open.filter((p) => choices[p.id]?.verdict === 'reject')
  const allReady = open.length > 0 && open.every((p) => ready(p, choices[p.id]))
  const approvals = open.filter((p) => choices[p.id]?.verdict === 'approve').length

  const confirm = async () => {
    let last: Plan = plan
    // Approvals first, in position order, so a task's dependencies get their issues before it.
    for (const p of open.filter((p) => choices[p.id].verdict === 'approve'))
      last = await act.mutateAsync({ path: `proposals/${p.id}/approve` })
    for (const p of rejected) {
      const c = choices[p.id] as Extract<Choice, { verdict: 'reject' }>
      last = await act.mutateAsync({ path: `proposals/${p.id}/deny`, body: { feedback: c.reason.trim(), outcome: c.outcome } })
    }
    setChoices({})
    if (last.status === 'done') navigate(`/projects/${projectId}/plans?tab=done`)
  }

  return (
    <section className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="eyebrow m-0 mr-auto">Proposed tasks</h2>
        {open.length > 1 && (
          <>
            <Button className="min-h-9 px-3 text-sm" disabled={busy} onClick={() => setChoices(Object.fromEntries(open.filter(approvable).map((p) => [p.id, { verdict: 'approve' }])))}>
              <Icon name="check" size={14} className="text-good" />
              Approve all
            </Button>
            <Button className="min-h-9 px-3 text-sm" disabled={busy} onClick={() => setChoices(Object.fromEntries(open.map((p) => [p.id, reject(p.id)])))}>
              <Icon name="close" size={14} className="text-critical" />
              Reject all
            </Button>
          </>
        )}
      </div>
      {rejected.length > 1 && (
        <TextInput
          aria-label="Reason for every rejected task"
          placeholder="One reason for every rejected task"
          className="min-h-9 text-sm"
          onChange={(e) => setChoices((all) => ({ ...all, ...Object.fromEntries(rejected.map((p) => [p.id, { ...reject(p.id), reason: e.target.value }])) }))}
        />
      )}
      <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
        {proposals.map((p) => {
          const c = choices[p.id]
          return (
            <li key={p.id} className="rounded-lg border border-line-strong bg-surface-sunken px-3 py-2">
              <div className="flex items-start gap-3">
                <details className="min-w-0 flex-1 wrap-anywhere">
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
                {decidable(p) && (
                  <div role="group" aria-label={`Decide on ${p.title}`} className="inline-flex flex-none overflow-hidden rounded-lg border border-line-strong bg-surface">
                    <button
                      type="button"
                      aria-label={`Approve ${p.title}`}
                      title="Approve, add to Backlog"
                      aria-pressed={c?.verdict === 'approve'}
                      disabled={busy || !approvable(p)}
                      onClick={() => choose(p.id, c?.verdict === 'approve' ? undefined : { verdict: 'approve' })}
                      className={`flex size-8 cursor-pointer items-center justify-center border-0 text-good hover:bg-good/25 disabled:cursor-not-allowed disabled:opacity-40 ${c?.verdict === 'approve' ? 'bg-good text-surface hover:bg-good' : 'bg-good/10'}`}
                    >
                      <Icon name="check" size={16} />
                    </button>
                    <button
                      type="button"
                      aria-label={`Reject ${p.title}`}
                      title="Reject"
                      aria-pressed={c?.verdict === 'reject'}
                      disabled={busy}
                      onClick={() => choose(p.id, c?.verdict === 'reject' ? undefined : reject(p.id))}
                      className={`flex size-8 cursor-pointer items-center justify-center border-0 border-l border-line-strong text-critical hover:bg-critical/25 disabled:cursor-not-allowed disabled:opacity-40 ${c?.verdict === 'reject' ? 'bg-critical text-surface hover:bg-critical' : 'bg-critical/10'}`}
                    >
                      <Icon name="close" size={16} />
                    </button>
                  </div>
                )}
              </div>
              {c?.verdict === 'reject' && (
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  <TextInput
                    aria-label={`Why reject ${p.title}`}
                    placeholder="Why? The agent revises from this."
                    value={c.reason}
                    onChange={(e) => choose(p.id, { ...c, reason: e.target.value })}
                    className="min-h-9 flex-1 text-sm"
                  />
                  <Select
                    aria-label={`What to do with ${p.title}`}
                    value={c.outcome}
                    onChange={(e) => choose(p.id, { ...c, outcome: e.target.value as 'revise' | 'drop' })}
                    className="min-h-9 text-sm"
                  >
                    <option value="drop">Drop it</option>
                    <option value="revise">Revise it</option>
                  </Select>
                </div>
              )}
            </li>
          )
        })}
      </ul>
      {open.length > 0 && (
        <div className="flex items-center justify-end gap-3">
          <span className="text-xs text-text-muted">
            {allReady
              ? `${approvals} to approve, ${rejected.length} to reject`
              : `Choose ✓ or ✗ for every task${rejected.length ? ', with a reason for each rejection' : ''}.`}
          </span>
          <Button variant="primary" className="min-h-9 px-4 text-sm" disabled={busy || !allReady} onClick={() => confirm().catch(() => {})}>
            Confirm
          </Button>
        </div>
      )}
    </section>
  )
}
