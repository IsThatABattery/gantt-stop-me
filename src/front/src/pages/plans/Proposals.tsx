import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { Chip } from '../../components/Chip'
import { Button, Select, TextInput } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { InkTab, Tile } from '../../components/Tile'
import type { Plan, Proposal, usePlanAction } from '../../data/views/plans'
import { labels } from '../../labels/labels'
import { Card } from '../task-drawer/Card'
import { proposalLook } from './look'

type Choice = { verdict: 'approve' } | { verdict: 'reject'; reason: string; outcome: 'revise' | 'drop' }

const decidable = (p: Proposal) => p.status === 'needs_review' || p.status === 'sent_back'
// A task sent back is with the agent until it returns it, so it can only be dropped or sent back again.
const approvable = (p: Proposal) => p.status === 'needs_review'
const ready = (p: Proposal, c?: Choice) => !!c && (c.verdict === 'approve' ? approvable(p) : !!c.reason.trim())

/** Approve and reject: a pair of square toggles. Each only chooses; Confirm sends every choice at once. */
const decide =
  'grid size-11 flex-none cursor-pointer place-items-center border-[1.5px] border-ink transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-45'

/**
 * The task agent's proposed tasks, as cards on a tile, as tasks sit on the board. Each is approved or rejected with a
 * reason; Confirm sends every choice at once, and a plan with nothing left to decide is done.
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
    <Tile
      step={1}
      as="section"
      aria-labelledby="proposals"
      frameClassName="gap-3"
      tab={
        <InkTab
          id="proposals"
          count={proposals.length}
          countLabel="tasks"
          mark={open.length > 0 ? 'approve' : undefined}
          markLabel={open.length > 0 ? `${open.length} to review` : undefined}
        >
          Proposed tasks
        </InkTab>
      }
    >
      <div className="flex flex-wrap items-center gap-2">
        <p className="m-0 mr-auto min-w-[14rem] flex-1 text-[13px] leading-normal text-ink-2">
          Approved tasks become issues in the {labels.backlog}. A rejected one is dropped, or sent back to the task agent with your reason.
        </p>
        {open.length > 1 && (
          <>
            <Button
              disabled={busy}
              onClick={() => setChoices(Object.fromEntries(open.filter(approvable).map((p) => [p.id, { verdict: 'approve' }])))}
            >
              <Icon name="check" size={16} />
              Approve all
            </Button>
            <Button disabled={busy} onClick={() => setChoices(Object.fromEntries(open.map((p) => [p.id, reject(p.id)])))}>
              <Icon name="close" size={16} />
              Reject all
            </Button>
          </>
        )}
      </div>
      {rejected.length > 1 && (
        <TextInput
          aria-label="Reason for every rejected task"
          placeholder="One reason for every rejected task"
          onChange={(e) => setChoices((all) => ({ ...all, ...Object.fromEntries(rejected.map((p) => [p.id, { ...reject(p.id), reason: e.target.value }])) }))}
        />
      )}

      <ol className="m-0 flex list-none flex-col gap-2 p-0">
        {proposals.map((p) => {
          const c = choices[p.id]
          const look = proposalLook[p.status]
          return (
            <Card key={p.id} as="li" tone={c?.verdict === 'reject' ? 'signal' : 'plain'}>
              <div className="flex items-start gap-3">
                <details className="group min-w-0 flex-1 wrap-anywhere">
                  <summary className="-m-1 flex min-h-11 cursor-pointer list-none items-start gap-2 rounded-[2px] p-1 hover:bg-ink/5 [&::-webkit-details-marker]:hidden">
                    <Icon name="chevronRight" size={16} className="mt-0.5 flex-none transition-transform duration-150 group-open:rotate-90" />
                    <span className="flex min-w-0 flex-col gap-1.5">
                      <span className="text-sm leading-snug font-semibold">
                        <span className="sr-only">Task {p.position}: </span>
                        {p.title}
                      </span>
                      <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-ink-2">
                        <Chip tone={look.tone} icon={look.icon} soft={look.tone !== 'approve'}>
                          {look.word}
                        </Chip>
                        {p.tests.length} {p.tests.length === 1 ? 'test' : 'tests'}
                      </span>
                    </span>
                  </summary>
                  <div className="flex flex-col gap-2.5 pt-2.5 pl-6">
                    {p.description && <p className="m-0 max-w-[68ch] text-[13px] leading-normal whitespace-pre-wrap text-ink">{p.description}</p>}
                    {p.acceptanceCriteria.length > 0 && (
                      <div>
                        <h4 className="m-0 text-xs font-semibold text-ink-2">Done when</h4>
                        <ul className="m-0 mt-1 flex list-disc flex-col gap-0.5 pl-4 text-[13px] leading-normal marker:text-ink-2">
                          {p.acceptanceCriteria.map((a, i) => (
                            <li key={i}>{a}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {p.tests.length > 0 && (
                      <div>
                        <h4 className="m-0 text-xs font-semibold text-ink-2">Tests</h4>
                        <ul className="m-0 mt-1 flex list-none flex-col gap-1 p-0 text-[13px] leading-normal">
                          {p.tests.map((t, i) => (
                            <li key={i} className="flex items-baseline gap-2">
                              <span className="w-[5.5rem] flex-none text-xs text-ink-2">{t.kind === 'script' ? 'Command' : 'LLM check'}</span>
                              {t.command ? <code className="font-mono text-xs break-all">{t.command}</code> : t.name}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {p.agentNote && <p className="m-0 text-[13px] leading-normal text-ink-2">Agent&rsquo;s note: {p.agentNote}</p>}
                  </div>
                </details>
                {p.taskId && (
                  <Link to={`/projects/${projectId}/tasks/${p.taskId}`} className="inline-flex min-h-11 flex-none items-center text-[13px] font-semibold">
                    Open task
                  </Link>
                )}
                {decidable(p) && (
                  <div role="group" aria-label={`Decide on ${p.title}`} className="flex flex-none">
                    <button
                      type="button"
                      aria-label={`Approve ${p.title}`}
                      title={`Approve, add to ${labels.backlog}`}
                      aria-pressed={c?.verdict === 'approve'}
                      disabled={busy || !approvable(p)}
                      onClick={() => choose(p.id, c?.verdict === 'approve' ? undefined : { verdict: 'approve' })}
                      className={`${decide} rounded-l-control ${c?.verdict === 'approve' ? 'bg-ink text-on-ink' : 'bg-paper text-ink enabled:hover:bg-ink/8'}`}
                    >
                      <Icon name="check" size={18} />
                    </button>
                    <button
                      type="button"
                      aria-label={`Reject ${p.title}`}
                      title="Reject"
                      aria-pressed={c?.verdict === 'reject'}
                      disabled={busy}
                      onClick={() => choose(p.id, c?.verdict === 'reject' ? undefined : reject(p.id))}
                      className={`${decide} -ml-[1.5px] rounded-r-control ${
                        c?.verdict === 'reject' ? 'border-signal bg-signal text-on-signal' : 'bg-paper text-ink enabled:hover:bg-ink/8'
                      }`}
                    >
                      <Icon name="close" size={18} />
                    </button>
                  </div>
                )}
              </div>
              {c?.verdict === 'reject' && (
                <div className="flex flex-wrap items-center gap-2 border-t border-keyline pt-2.5">
                  <TextInput
                    aria-label={`Why reject ${p.title}`}
                    placeholder="Why? The agent revises from this."
                    value={c.reason}
                    onChange={(e) => choose(p.id, { ...c, reason: e.target.value })}
                    className="min-w-[12rem] flex-1"
                  />
                  <Select
                    aria-label={`What to do with ${p.title}`}
                    value={c.outcome}
                    onChange={(e) => choose(p.id, { ...c, outcome: e.target.value as 'revise' | 'drop' })}
                  >
                    <option value="drop">Drop it</option>
                    <option value="revise">Revise it</option>
                  </Select>
                </div>
              )}
            </Card>
          )
        })}
      </ol>

      {open.length > 0 && (
        <div className="flex flex-wrap items-center justify-end gap-x-4 gap-y-2 border-t border-tile-keyline pt-3">
          <span role="status" className="text-[13px] text-ink-2">
            {allReady
              ? `${approvals} to approve, ${rejected.length} to reject.`
              : `Approve or reject every task${rejected.length ? ', with a reason for each rejection' : ''}.`}
          </span>
          <Button variant="primary" busy={act.isPending} disabled={busy || !allReady} onClick={() => confirm().catch(() => {})}>
            Confirm
          </Button>
        </div>
      )}
    </Tile>
  )
}
