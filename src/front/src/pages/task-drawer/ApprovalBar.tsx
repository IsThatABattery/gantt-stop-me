import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { Button, Textarea } from '../../components/controls'
import { signedInUserId } from '../../data/mock/store'
import type { Task } from '../../data/types'
import { useApproveTask, useRequestChanges, useTakeOver, type TaskDetail } from '../../data/views/taskDrawer'
import { actionLabels, columnLabels } from '../../labels/labels'

interface ApprovalBarProps {
  detail: TaskDetail
  /** Called with a short confirmation after an action succeeds */
  onDone: (message: string) => void
}

/** True when keys typed here belong to a field, not to the shortcuts. */
function isTyping(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

/** Pinned bar for an agent's task waiting on review (PayloadDetail.dc.html), with A / R / T shortcuts. Mock data only. */
export function ApprovalBar({ detail, onDone }: ApprovalBarProps) {
  const { task, assignee } = detail
  const ref = { projectId: task.projectId, taskId: task.id }
  const approve = useApproveTask(task.projectId, task.id)
  const requestChanges = useRequestChanges(task.projectId, task.id)
  const takeOver = useTakeOver(task.projectId, task.id)
  const busy = approve.isPending || requestChanges.isPending || takeOver.isPending
  const error = approve.error ?? requestChanges.error ?? takeOver.error

  const [noting, setNoting] = useState(false)
  const [note, setNote] = useState('')
  const [noteError, setNoteError] = useState<string | null>(null)
  const noteId = useId()
  const focusNote = () => document.getElementById(noteId)?.focus()

  const alreadyMine = assignee?.kind === 'user' && assignee.user.id === signedInUserId
  const who = assignee ? (assignee.kind === 'agent' ? assignee.agent.name : assignee.user.name) : 'This task'

  const actions = {
    approve: () => approve.mutate(ref, { onSuccess: (t) => onDone(`${t.key} approved and merged. It is in ${columnLabels.closed.name}.`) }),
    request: () => {
      setNoting(true)
      setNoteError(null)
    },
    takeOver: () => {
      if (!alreadyMine) takeOver.mutate(ref, { onSuccess: (t) => onDone(`You took over ${t.key}.`) })
    },
  }
  const latest = useRef(actions)
  useEffect(() => {
    latest.current = actions
  })

  useEffect(() => {
    if (noting) document.getElementById(noteId)?.focus()
  }, [noting, noteId])

  // A / R / T while the drawer is open and focus isn't in a field (AGENT_BRIEF §10).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (busy || e.repeat || e.ctrlKey || e.metaKey || e.altKey || isTyping(e.target)) return
      const key = e.key.toLowerCase()
      const run = key === 'a' ? latest.current.approve : key === 'r' ? latest.current.request : key === 't' ? latest.current.takeOver : null
      if (!run) return
      e.preventDefault()
      run()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [busy])

  const sendBack = (e: FormEvent) => {
    e.preventDefault()
    if (!note.trim()) {
      setNoteError('Add a note so the assignee knows what to change.')
      focusNote()
      return
    }
    requestChanges.mutate(
      { ...ref, note },
      { onSuccess: (t: Task) => onDone(`Changes requested. ${t.key} is back in ${columnLabels['in-progress'].name}.`) },
    )
  }

  return (
    <div className="flex flex-col gap-3 border-t border-approve bg-approve/6 px-6 pt-4 pb-5">
      <p className="m-0 text-[13px]">{who} is waiting for your call on this task.</p>

      {noting ? (
        <form onSubmit={sendBack} className="flex flex-col gap-2.5" noValidate>
          <label htmlFor={noteId} className="text-[13px] text-ink">
            What should change? (required)
          </label>
          <Textarea
            id={noteId}
            rows={3}
            value={note}
            onChange={(e) => {
              setNote(e.target.value)
              if (noteError) setNoteError(null)
            }}
            aria-invalid={!!noteError}
            aria-describedby={noteError ? `${noteId}-error` : undefined}
          />
          {noteError && (
            <p id={`${noteId}-error`} role="alert" className="m-0 text-[13px] text-signal-ink">
              {noteError}
            </p>
          )}
          <div className="flex flex-wrap gap-2.5">
            <Button type="submit" disabled={busy} className="flex-[1_1_160px] border-approve text-approve-ink hover:border-approve hover:text-approve-ink">
              Send back for changes
            </Button>
            <Button
              onClick={() => {
                setNoting(false)
                setNote('')
                setNoteError(null)
              }}
            >
              {actionLabels.cancel}
            </Button>
          </div>
        </form>
      ) : (
        <div className="flex flex-wrap gap-2.5">
          <Button variant="primary" disabled={busy} onClick={actions.approve} className="flex-[1_1_160px]">
            {actionLabels.approveLand}
          </Button>
          <Button
            disabled={busy}
            onClick={actions.request}
            className="flex-[1_1_140px] border-approve font-medium text-approve-ink hover:border-approve hover:text-approve-ink"
          >
            {actionLabels.requestChanges}
          </Button>
          {!alreadyMine && (
            <Button disabled={busy} onClick={actions.takeOver} className="flex-[0_1_110px]">
              {actionLabels.takeOver}
            </Button>
          )}
        </div>
      )}

      {error && (
        <p role="alert" className="m-0 text-[13px] text-signal-ink">
          {error.message}
        </p>
      )}
      <p className="m-0 font-mono text-xs text-ink-2" aria-hidden="true">
        {alreadyMine ? 'A approve · R request changes' : actionLabels.shortcuts}
      </p>
      <p className="sr-only">Keyboard shortcuts: {alreadyMine ? 'A approve, R request changes.' : 'A approve, R request changes, T take over.'}</p>
    </div>
  )
}
