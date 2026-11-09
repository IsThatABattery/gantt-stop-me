import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Chip, type Tone } from '../../components/Chip'
import { Button } from '../../components/controls'
import { Icon, type IconName } from '../../components/Icon'
import { runLook } from '../../components/runLook'
import type { TaskDetail, TaskTest } from '../../data/api/types'
import { useRetest, useRun, useRuns, type RunResult } from '../../data/views/runs'
import { actionLabels, runStatusLabels } from '../../labels/labels'
import { Card, type CardTone } from './Card'
import { isLive } from './state'
import { splitTestText } from './testText'

/** The Tests tab: the latest test run, each test with its result, and the run's logs. */
export function TestRuns({ projectId, task }: { projectId: string; task: TaskDetail }) {
  const taskId = String(task.id)
  const runs = useRuns(projectId, taskId)
  const latest = runs.data?.[0]
  const run = useRun(projectId, latest?.id)
  const retest = useRetest(projectId, taskId)
  const live = isLive(latest?.status)
  const results = run.data?.results ?? []
  const media = `/api/projects/${projectId}/runs/${latest?.id}/results`

  return (
    <div className="flex flex-col gap-2.5 pt-3">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="min-w-0">
          <h2 className="m-0 text-sm font-bold">{latest ? 'Latest test run' : 'No test run yet'}</h2>
          <p className="m-0 text-[13px] text-ink-2">
            {latest
              ? `${latest.prNumber ? `PR #${latest.prNumber}` : 'No linked pull request'} · ${runStatusLabels[latest.status]}`
              : 'A test run checks the linked pull request in its own containers.'}
          </p>
        </div>
        <Button busy={retest.isPending} onClick={() => retest.mutate(undefined)}>
          {latest ? (live ? 'Restart test run' : 'Retest') : 'Run tests'}
        </Button>
      </div>

      {retest.error && <Problem alert>{retest.error.message}</Problem>}
      {latest?.reason && latest.reason !== task.needsAttention && <Problem>{latest.reason}</Problem>}
      {latest?.approvedBy && (
        <p className="m-0 flex items-center gap-1.5 text-[13px] font-semibold">
          <Icon name="check" size={16} className="text-good" />
          Approved and merged by {latest.approvedBy}
        </p>
      )}
      {task.testsError && <Problem title="The Tests section could not be read">{task.testsError}</Problem>}

      {results.length > 0 ? (
        <ol className="m-0 flex list-none flex-col gap-2 p-0">
          {results.map((r) => (
            <ResultCard key={r.id} result={r} live={live} media={media} />
          ))}
        </ol>
      ) : task.tests.length > 0 ? (
        <ol className="m-0 flex list-none flex-col gap-2 p-0">
          {task.tests.map((t, i) => (
            <TestCard key={i} test={t} status={latest && !run.isPending ? (live ? 'running' : 'none') : latest ? 'loading' : 'none'} />
          ))}
        </ol>
      ) : (
        <div className="flex flex-col items-center gap-1.5 rounded-[3px] border border-ink/25 px-5 py-7 text-center">
          <p className="m-0 text-[13px] font-bold">No tests yet</p>
          <p className="m-0 max-w-[44ch] text-[13px] leading-normal text-ink-2">
            Tests live in the issue&rsquo;s description, under a <code className="font-mono text-xs text-ink">## Tests</code> heading: each item is a
            command to run or a plain-English check.
          </p>
          <Link to={`/projects/${projectId}/tasks/${task.id}/edit`} className={buttonClass('link')}>
            {actionLabels.editTask}
          </Link>
        </div>
      )}

      {run.data && run.data.executions.length > 0 && (
        <section className="mt-2 flex flex-col gap-1.5">
          <h3 className="m-0 text-[13px] font-semibold text-ink-2">Logs</h3>
          {run.data.executions.map((e) => {
            const look = runLook[e.status]
            return (
              <details key={e.id} className="group min-w-0 rounded-card border border-line bg-paper">
                <summary className="flex min-h-11 cursor-pointer list-none items-center gap-2 rounded-card px-3 text-[13px] font-semibold hover:bg-ink/5 [&::-webkit-details-marker]:hidden">
                  <Icon name="chevronRight" size={16} className="flex-none transition-transform duration-150 group-open:rotate-90" />
                  <span className="min-w-0 flex-1 truncate">{e.workflow} log</span>
                  <Chip tone={look.tone} icon={look.icon} iconClassName={look.iconClass} soft>
                    {runStatusLabels[e.status]}
                  </Chip>
                </summary>
                <pre className="m-0 max-h-72 overflow-auto border-t border-keyline px-3 py-2.5 font-mono text-xs leading-relaxed whitespace-pre-wrap text-ink">
                  {e.logTail || 'Nothing logged.'}
                </pre>
              </details>
            )
          })}
        </section>
      )}
    </div>
  )
}

/** A problem with the run or its tests. alert announces it, for one that just happened. */
function Problem({ title, alert, children }: { title?: string; alert?: boolean; children: string }) {
  return (
    <div role={alert ? 'alert' : undefined} className="flex gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] leading-normal text-signal-ink">
      <Icon name="alert" size={16} className="mt-px flex-none" />
      <div className="min-w-0 break-words">
        {title && <p className="m-0 font-bold">{title}</p>}
        <p className="m-0">{children}</p>
      </div>
    </div>
  )
}

/** A result's chip: never colour alone, always an icon and a word. Pending reads as running while the run is live. */
const resultLook: Record<RunResult['status'], { tone: Tone; icon: IconName; word: string; card: CardTone }> = {
  pending: { tone: 'neutral', icon: 'queued', word: 'Waiting', card: 'plain' },
  passed: { tone: 'good', icon: 'check', word: 'Passed', card: 'plain' },
  failed: { tone: 'signal', icon: 'failed', word: 'Failed', card: 'signal' },
  manual: { tone: 'approve', icon: 'person', word: 'Check by hand', card: 'approve' },
  error: { tone: 'signal', icon: 'error', word: 'Runner error', card: 'signal' },
  skipped: { tone: 'neutral', icon: 'cancelled', word: 'Skipped', card: 'plain' },
}

function ResultCard({ result: r, live, media }: { result: RunResult; live: boolean; media: string }) {
  const look = r.status === 'pending' && live ? { ...resultLook.pending, tone: 'live' as Tone, icon: 'running' as IconName, word: 'Running' } : resultLook[r.status]
  const { head, command, body, code } = splitTestText(r.text)
  const pick = Object.entries(r.probabilities).sort((a, b) => b[1] - a[1])[0]
  const meta = [
    r.kind === 'script' ? 'Command' : 'LLM check',
    r.workflow && `${r.workflow} workflow`,
    r.workflow && pick && `picked ${Math.round(pick[1] * 100)}% sure`,
  ].filter(Boolean)

  return (
    <Card as="li" tone={look.card}>
      <div className="flex items-start justify-between gap-3">
        <p className={`m-0 min-w-0 pt-0.5 break-words ${command ? 'font-mono text-[13px] font-semibold' : 'text-sm font-semibold'}`}>{head}</p>
        <Chip
          tone={look.tone}
          icon={look.icon}
          iconClassName={look.icon === 'running' ? 'motion-safe:animate-live' : undefined}
          className="flex-none"
        >
          {look.word}
        </Chip>
      </div>
      {body && <Block code={code}>{body}</Block>}
      <p className="m-0 text-xs text-ink-2">{meta.join(' · ')}</p>
      {r.detail &&
        (r.kind === 'script' ? (
          <pre className="m-0 max-h-48 overflow-auto rounded-[2px] bg-ground px-2.5 py-2 font-mono text-xs leading-relaxed whitespace-pre-wrap text-ink">{r.detail}</pre>
        ) : (
          <p className="m-0 text-[13px] leading-normal break-words whitespace-pre-wrap text-ink">{r.detail}</p>
        ))}
      {r.hasScreenshot && (
        <img src={`${media}/${r.id}/screenshot`} alt={`Screenshot for ${head}`} className="max-w-full rounded-[2px] border border-keyline" />
      )}
      {r.hasVideo && <video src={`${media}/${r.id}/video`} controls className="max-w-full rounded-[2px] border border-keyline" />}
    </Card>
  )
}

/** One of the issue's tests, before any run has results for it. */
function TestCard({ test, status }: { test: TaskTest; status: 'none' | 'running' | 'loading' }) {
  const command = test.kind === 'script' && test.command === test.name
  const meta = [test.kind === 'script' ? 'Command' : 'LLM check', test.workflow && `${test.workflow} workflow`].filter(Boolean)
  return (
    <Card as="li">
      <div className="flex items-start justify-between gap-3">
        <p className={`m-0 min-w-0 pt-0.5 break-words ${command ? 'font-mono text-[13px] font-semibold' : 'text-sm font-semibold'}`}>{test.name}</p>
        {status === 'running' ? (
          <Chip tone="live" icon="running" iconClassName="motion-safe:animate-live" className="flex-none">
            Running
          </Chip>
        ) : status === 'none' ? (
          <Chip tone="neutral" soft icon="idle" className="flex-none">
            Not run
          </Chip>
        ) : null}
      </div>
      {test.kind === 'script' && !command && <Block code>{test.command}</Block>}
      {test.kind === 'check' && test.details && <Block code={false}>{test.details}</Block>}
      <p className="m-0 text-xs text-ink-2">{meta.join(' · ')}</p>
    </Card>
  )
}

function Block({ code, children }: { code: boolean; children: string }) {
  return code ? (
    <pre className="m-0 overflow-x-auto rounded-[2px] bg-ground px-2.5 py-2 font-mono text-xs leading-relaxed text-ink">{children}</pre>
  ) : (
    <p className="m-0 text-[13px] leading-normal whitespace-pre-wrap text-ink-2">{children}</p>
  )
}
