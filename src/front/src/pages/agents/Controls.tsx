import { useId, useState, type FormEvent } from 'react'
import { AutonomyRings } from '../../components/Avatar'
import { Button, TextInput } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { RadioCard } from '../../components/RadioCard'
import type { Autonomy, User } from '../../data/types'
import { cx } from '../../lib/cx'
import { useRecallAgent, useRedirectAgent, useSetAutonomy, useSetPaused, type AgentRow } from '../../data/views/agents'
import { actionLabels, autonomyDescriptions, autonomyLabels, labels } from '../../labels/labels'

// Red outline button. Not Button: its default border and text colors would win over red.
const danger = (extra = '') =>
  cx('inline-flex min-h-11 cursor-pointer items-center justify-center gap-2 rounded-control border border-critical bg-transparent px-4 font-[inherit] font-medium text-critical hover:bg-critical/10 disabled:cursor-not-allowed disabled:opacity-50', extra)

interface ControlsProps {
  row: AgentRow
  projectId: string
  me: User | undefined
  freeFlightGrantedBy: 'lead' | 'any' | undefined
}

/** Pause, redirect, autonomy and recall for the selected astronaut (AGENT_BRIEF §6). */
export function Controls({ row, projectId, me, freeFlightGrantedBy }: ControlsProps) {
  const { agent, task } = row
  const ref = { projectId, agentId: agent.id }
  const pause = useSetPaused(projectId)
  const redirect = useRedirectAgent(projectId)
  const autonomy = useSetAutonomy(projectId)
  const recall = useRecallAgent(projectId)
  const [mode, setMode] = useState<'idle' | 'redirect' | 'recall'>('idle')
  const [instruction, setInstruction] = useState('')
  const [problem, setProblem] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const id = useId()

  const canGrantFreeFlight = freeFlightGrantedBy === 'any' || me?.role === 'lead'
  const error = pause.error ?? redirect.error ?? autonomy.error ?? recall.error

  const sendRedirect = (e: FormEvent) => {
    e.preventDefault()
    if (!instruction.trim()) {
      setProblem('Type an instruction first.')
      document.getElementById(`${id}-redirect`)?.focus()
      return
    }
    redirect.mutate(
      { ...ref, instruction },
      {
        onSuccess: () => {
          setNotice(`Instruction logged for ${agent.name}.`)
          setInstruction('')
          setMode('idle')
        },
      },
    )
  }

  return (
    <div className="flex flex-[1_1_260px] flex-col gap-3 rounded-card border border-line bg-surface-sunken p-5">
      <h3 className="m-0 mb-1 font-heading text-[15px] font-semibold">Controls</h3>

      <Button
        aria-pressed={agent.paused}
        disabled={pause.isPending}
        onClick={() => pause.mutate({ ...ref, paused: !agent.paused }, { onSuccess: (a) => setNotice(`${a.name} ${a.paused ? 'paused' : 'resumed'}.`) })}
      >
        <Icon name={agent.paused ? 'rocket' : 'pause'} size={16} />
        {agent.paused ? actionLabels.resume : actionLabels.pause}
      </Button>

      {mode === 'redirect' ? (
        <form onSubmit={sendRedirect} noValidate className="flex flex-col gap-2 rounded-control border border-line p-3">
          <label htmlFor={`${id}-redirect`} className="text-[13px] text-text-soft">
            New instruction for {agent.name}
          </label>
          <TextInput
            id={`${id}-redirect`}
            autoFocus
            value={instruction}
            aria-invalid={!!problem}
            onChange={(e) => {
              setInstruction(e.target.value)
              setProblem(null)
            }}
          />
          {problem && (
            <p role="alert" className="m-0 text-[13px] text-critical">
              {problem}
            </p>
          )}
          <div className="flex gap-2">
            <Button type="submit" variant="primary" disabled={redirect.isPending} className="flex-1">
              Send
            </Button>
            <Button onClick={() => setMode('idle')}>{actionLabels.cancel}</Button>
          </div>
        </form>
      ) : (
        <Button
          disabled={!task}
          onClick={() => {
            setMode('redirect')
            setNotice(null)
          }}
        >
          {actionLabels.redirect}
        </Button>
      )}

      <fieldset className="m-0 mt-2 flex flex-col gap-2 border-0 p-0">
        <legend className="pb-2 text-xs text-text-muted">{actionLabels.changeAutonomy}</legend>
        {([0, 1, 2] as Autonomy[]).map((level) => {
          const locked = level === 2 && !canGrantFreeFlight
          return (
            <RadioCard
              key={level}
              tone="astronaut"
              name={`${id}-autonomy`}
              value={level}
              checked={agent.autonomy === level}
              disabled={locked || autonomy.isPending}
              onChange={() =>
                autonomy.mutate({ ...ref, autonomy: level }, { onSuccess: (a) => setNotice(`${a.name} is now ${autonomyLabels[a.autonomy]}.`) })
              }
              className="p-3"
              label={
                <>
                  {autonomyLabels[level]}
                  <AutonomyRings level={level} />
                </>
              }
              description={locked ? `Only the ${labels.lead.toLowerCase()} can grant this.` : autonomyDescriptions[level]}
            />
          )
        })}
      </fieldset>

      {mode === 'recall' && task ? (
        <div role="alertdialog" aria-labelledby={`${id}-recall`} className="mt-auto flex flex-col gap-3 rounded-control border border-critical bg-critical/8 p-3">
          <p id={`${id}-recall`} className="m-0 text-[13px] leading-normal">
            Recall {agent.name}? {task.key} goes back to the {labels.backlog} and {agent.name} stops work on it.
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              disabled={recall.isPending}
              onClick={() =>
                recall.mutate(ref, {
                  onSuccess: () => {
                    setNotice(`${task.key} is back in the ${labels.backlog}.`)
                    setMode('idle')
                  },
                })
              }
              className={danger('flex-1')}
            >
              Recall
            </button>
            <Button onClick={() => setMode('idle')}>{actionLabels.cancel}</Button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          disabled={!task}
          onClick={() => {
            setMode('recall')
            setNotice(null)
          }}
          className={danger('mt-auto')}
        >
          {actionLabels.recall}
        </button>
      )}

      <div role="status" aria-live="polite" className="text-[13px]">
        {notice && !error && <span className="text-good">{notice}</span>}
        {error && <span className="text-critical">{error.message}</span>}
      </div>
    </div>
  )
}
