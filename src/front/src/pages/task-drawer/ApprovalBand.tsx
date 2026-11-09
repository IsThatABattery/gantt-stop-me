import { useEffect, useId, useState, type FormEvent } from 'react'
import { Checkbox, Textarea } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { UnsavedChangesGuard } from '../../components/UnsavedChangesGuard'
import type { TaskDetail } from '../../data/api/types'
import { NotFoundError } from '../../data/errors'
import { useRun, useRuns, type Run, type useApprove, type useRequestChanges } from '../../data/views/runs'
import { actionLabels, columnLabels } from '../../labels/labels'
import { band, bandMark, bandPrimary, bandSecondary } from './band'
import { splitTestText } from './testText'

interface ApprovalBandProps {
  projectId: string
  task: TaskDetail
  /**
   * The drawer's own approve and request changes. They outlive the band: a failed approve can refetch the task out of
   * User Check, taking the band along, and the drawer still shows why. The drawer confirms a success.
   */
  approve: ReturnType<typeof useApprove>
  changes: ReturnType<typeof useRequestChanges>
}

/** Why Approve is not open yet, from the latest test run. */
function blockedBy(latest: Run | undefined, loading: boolean, failed: boolean): string | null {
  if (loading) return 'Loading the latest test run…'
  if (failed) return 'The latest test run didn’t load. Try again from the Tests tab.'
  if (!latest) return 'Approve and merge needs a passed test run, and this task has none yet.'
  switch (latest.status) {
    case 'queued':
    case 'running':
      return 'Tests are running. Approve once they pass.'
    case 'failed':
      return 'The latest test run failed. Request changes, or retest from the Tests tab.'
    case 'error':
      return 'The latest test run broke before it could check the code. Retest from the Tests tab.'
    case 'cancelled':
      return 'The latest test run was cancelled. Retest from the Tests tab.'
    case 'passed':
      return null
  }
}

/**
 * The yellow band pinned to the foot of the drawer while a task waits in User Check: the manual tests to tick, then
 * Approve and merge, or Request changes with a note for the pull request.
 */
export function ApprovalBand({ projectId, task, approve, changes }: ApprovalBandProps) {
  const taskId = String(task.id)
  const runs = useRuns(projectId, taskId)
  const latest = runs.data?.[0]
  const run = useRun(projectId, latest?.id)
  const [checked, setChecked] = useState<number[]>([])
  const [noting, setNoting] = useState(false)
  const [note, setNote] = useState('')
  const [noteError, setNoteError] = useState<string | null>(null)
  const id = useId()

  const manual = run.data?.results.filter((r) => r.status === 'manual') ?? []
  // A 404 means the API has no runner, so no run: that is "none yet", not a failure.
  const runsFailed = (!!runs.error && !runs.data && !(runs.error instanceof NotFoundError)) || (!!run.error && !run.data)
  const blocked = blockedBy(latest, runs.isPending || (latest !== undefined && run.isPending), runsFailed)
  const unticked = manual.filter((r) => !checked.includes(r.id)).length
  const canApprove = !blocked && unticked === 0
  const busy = approve.isPending || changes.isPending

  useEffect(() => {
    if (noting) document.getElementById(`${id}-note`)?.focus()
  }, [noting, id])

  const sendBack = (e: FormEvent) => {
    e.preventDefault()
    if (!note.trim()) {
      setNoteError('Say what should change, so the author knows what to fix.')
      document.getElementById(`${id}-note`)?.focus()
      return
    }
    // Only the last action's error shows, under its own button.
    approve.reset()
    changes.mutate(note.trim())
  }

  const summary = blocked
    ? blocked
    : unticked > 0
      ? `Tests passed${latest?.prNumber ? ` on PR #${latest.prNumber}` : ''}. Tick ${unticked === 1 ? 'the manual test' : `the ${unticked} manual tests`} once you have checked ${unticked === 1 ? 'it' : 'them'} by hand.`
      : `Tests passed${latest?.prNumber ? ` on PR #${latest.prNumber}` : ''}. Approving merges it and closes the task.`

  return (
    <section aria-labelledby={`${id}-title`} className={`flex flex-col gap-3 px-5 pt-4 pb-5 sm:px-6 ${band}`}>
      <div className="flex items-start gap-3">
        <span className={bandMark} aria-hidden="true">
          <Icon name="person" size={18} />
        </span>
        <div className="min-w-0">
          <h2 id={`${id}-title`} className="m-0 text-[15px] leading-tight font-bold">
            Check and approve
          </h2>
          <p className="m-0 mt-0.5 text-[13px] leading-normal">{summary}</p>
        </div>
      </div>

      {noting ? (
        <form onSubmit={sendBack} noValidate className="flex flex-col gap-2">
          <label htmlFor={`${id}-note`} className="text-[13px] font-bold">
            What should change?
          </label>
          <Textarea
            id={`${id}-note`}
            rows={3}
            value={note}
            onChange={(e) => {
              setNote(e.target.value)
              if (noteError) setNoteError(null)
            }}
            aria-invalid={!!noteError}
            aria-describedby={`${id}-note-help${noteError ? ` ${id}-note-error` : ''}`}
          />
          <p id={`${id}-note-help`} className="m-0 text-xs">
            Posted on the pull request as a comment. The task goes back to {columnLabels['in-progress'].name}.
          </p>
          {noteError && <BandError id={`${id}-note-error`}>{noteError}</BandError>}
          <div className="mt-1 flex flex-wrap gap-2.5">
            <button type="submit" className={`${bandPrimary} flex-[1_1_180px]`} disabled={busy} aria-busy={changes.isPending || undefined}>
              {changes.isPending ? 'Sending back…' : 'Send back for changes'}
            </button>
            <button
              type="button"
              className={bandSecondary}
              disabled={busy}
              onClick={() => {
                setNoting(false)
                setNote('')
                setNoteError(null)
              }}
            >
              {actionLabels.cancel}
            </button>
          </div>
          {changes.error && <BandError>{changes.error.message}</BandError>}
          {/* Esc or a click outside closes the drawer; ask before dropping the note. The prompt is paper, so it takes
              the page's ink rather than the band's. */}
          <div className="contents text-ink">
            <UnsavedChangesGuard when={!!note.trim() && !changes.isPending && !changes.isSuccess} />
          </div>
        </form>
      ) : (
        <>
          {manual.length > 0 && (
            <div role="group" aria-label="Manual tests" className="flex max-h-48 flex-col overflow-y-auto rounded-[3px] bg-paper px-3 py-0.5">
              {manual.map((r) => {
                const { head, body } = splitTestText(r.text)
                return (
                  <Checkbox
                    key={r.id}
                    className="border-t border-keyline py-1 first:border-t-0"
                    label={
                      <span className="flex min-w-0 flex-col">
                        <span className="font-semibold">{head}</span>
                        {body && <span className="text-xs whitespace-pre-wrap text-ink-2">{body}</span>}
                      </span>
                    }
                    checked={checked.includes(r.id)}
                    disabled={busy}
                    onChange={(e) => setChecked(e.target.checked ? [...checked, r.id] : checked.filter((x) => x !== r.id))}
                  />
                )
              })}
            </div>
          )}
          <div className="flex flex-wrap gap-2.5">
            <button
              type="button"
              className={`${bandPrimary} flex-[1_1_180px]`}
              disabled={!canApprove || busy}
              aria-busy={approve.isPending || undefined}
              onClick={() => {
                changes.reset()
                approve.mutate(checked)
              }}
            >
              <Icon name="check" size={16} />
              {approve.isPending ? 'Merging…' : actionLabels.approveLand}
            </button>
            <button type="button" className={`${bandSecondary} flex-[0_1_auto]`} disabled={busy} onClick={() => setNoting(true)}>
              {actionLabels.requestChanges}
            </button>
          </div>
          {approve.error && <BandError>{approve.error.message}</BandError>}
        </>
      )}
    </section>
  )
}

/** An error on the band: a signal square and the message, in the band's ink so it reads on yellow in every theme. */
function BandError({ id, children }: { id?: string; children: string }) {
  return (
    <p id={id} role="alert" className="m-0 flex items-start gap-2 text-[13px] leading-normal font-semibold">
      <span className="grid size-5 flex-none place-items-center rounded-[2px] bg-signal text-on-signal" aria-hidden="true">
        <Icon name="alert" size={12} />
      </span>
      <span className="min-w-0 wrap-anywhere">{children}</span>
    </p>
  )
}
