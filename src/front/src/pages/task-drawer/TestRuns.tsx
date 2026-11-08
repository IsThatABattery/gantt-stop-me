import { useState } from 'react'
import { Button, Checkbox, Textarea } from '../../components/controls'
import type { TaskDetail } from '../../data/api/types'
import { useApprove, useRequestChanges, useRetest, useRun, useRuns } from '../../data/views/runs'

/** The card's tests, its latest test run with the decision model's picks, and the approver's controls in User Check. */
export function TestRuns({ projectId, task, inUserCheck }: { projectId: string; task: TaskDetail; inUserCheck: boolean }) {
  const taskId = String(task.id)
  const runs = useRuns(projectId, taskId)
  const latest = runs.data?.[0]
  const run = useRun(projectId, latest?.id)
  const approve = useApprove(projectId, taskId)
  const changes = useRequestChanges(projectId, taskId)
  const retest = useRetest(projectId, taskId)
  const running = latest?.status === 'queued' || latest?.status === 'running'
  const [checked, setChecked] = useState<number[]>([])
  const [feedback, setFeedback] = useState('')
  const manual = run.data?.results.filter((r) => r.status === 'manual') ?? []
  const base = `/api/projects/${projectId}/runs/${latest?.id}/results`

  return (
    <>
      <section className="flex flex-col gap-2">
        <h2 className="eyebrow m-0">Tests</h2>
        {task.testsError && <p className="m-0 text-sm text-critical">{task.testsError}</p>}
        {task.tests.length === 0 ? (
          <p className="m-0 text-[13px] text-text-muted">No tests yet.</p>
        ) : (
          <ul className="m-0 text-sm text-text-soft">
            {task.tests.map((t, i) => (
              <li key={i}>
                {t.name} <span className="text-text-muted">({t.kind}{t.workflow ? `, ${t.workflow}` : ''})</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <div className="flex items-center gap-3">
        <Button className="min-h-9 px-3 text-sm" disabled={retest.isPending} onClick={() => retest.mutate(undefined)}>
          {latest ? (running ? 'Restart test run' : 'Retest') : 'Run tests'}
        </Button>
        {retest.error && (
          <span role="alert" className="text-[13px] text-critical">
            {retest.error.message}
          </span>
        )}
      </div>

      {latest && (
        <section className="flex flex-col gap-2">
          <h2 className="eyebrow m-0">
            Latest test run: {latest.status}
            {latest.prNumber && ` · PR #${latest.prNumber}`}
          </h2>
          {latest.reason && <p className="m-0 text-sm text-critical">{latest.reason}</p>}
          {latest.approvedBy && <p className="m-0 text-sm text-good">Approved and merged by {latest.approvedBy}</p>}
          {run.data?.results.map((r) => {
            const pick = Object.entries(r.probabilities).sort((a, b) => b[1] - a[1])[0]
            return (
              <div key={r.id} className="rounded-lg border border-line p-3 text-sm">
                <div>
                  <strong>{r.status}</strong> · {r.text}
                </div>
                {r.workflow && (
                  <div className="text-text-muted">
                    {r.kind} test in workflow <code>{r.workflow}</code>
                    {pick && ` (decision model: ${Math.round(pick[1] * 100)}% sure)`}
                  </div>
                )}
                {r.detail && <pre className="m-0 max-h-40 overflow-auto text-xs whitespace-pre-wrap text-text-soft">{r.detail}</pre>}
                {r.hasScreenshot && <img src={`${base}/${r.id}/screenshot`} alt={`Screenshot for ${r.text}`} className="mt-2 max-w-full" />}
                {r.hasVideo && <video src={`${base}/${r.id}/video`} controls className="mt-2 max-w-full" />}
              </div>
            )
          })}
          {run.data?.executions.map((e) => (
            <details key={e.id} className="text-sm">
              <summary>
                Log: {e.workflow} ({e.status})
              </summary>
              <pre className="max-h-60 overflow-auto text-xs whitespace-pre-wrap">{e.logTail}</pre>
            </details>
          ))}
        </section>
      )}

      {inUserCheck && latest?.status === 'passed' && !latest.approvedAt && (
        <section className="flex flex-col gap-2 rounded-lg border border-ion p-4">
          <h2 className="eyebrow m-0">Your check</h2>
          {manual.map((r) => (
            <Checkbox
              key={r.id}
              label={r.text}
              checked={checked.includes(r.id)}
              onChange={(e) => setChecked(e.target.checked ? [...checked, r.id] : checked.filter((id) => id !== r.id))}
            />
          ))}
          {(approve.error ?? changes.error) && (
            <p role="alert" className="m-0 text-sm text-critical">
              {(approve.error ?? changes.error)!.message}
            </p>
          )}
          <Button
            variant="primary"
            disabled={approve.isPending || checked.length < manual.length}
            onClick={() => approve.mutate(checked)}
            className="self-start"
          >
            Approve, merge and close
          </Button>
          <Textarea aria-label="What to change" placeholder="What to change" value={feedback} onChange={(e) => setFeedback(e.target.value)} />
          <Button disabled={changes.isPending || !feedback.trim()} onClick={() => changes.mutate(feedback.trim())} className="self-start">
            Request changes
          </Button>
        </section>
      )}
    </>
  )
}
