import { useId, useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { Chip } from '../../components/Chip'
import { Button, TextInput } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { useAnswerHail, type AgentRow } from '../../data/views/agents'
import { actionLabels, agentStateLabels, labels } from '../../labels/labels'
import { clockTime, stateLook, traceWord } from './look'

/** Selected astronaut: its payload, an inline hail reply, and the mono trace. */
export function AgentPanel({ row, projectId }: { row: AgentRow; projectId: string }) {
  const { agent, task, openHail, trace } = row
  const look = stateLook[agent.state]
  const answer = useAnswerHail(projectId)
  const [reply, setReply] = useState('')
  const [problem, setProblem] = useState<string | null>(null)
  const [sent, setSent] = useState<string | null>(null)
  const id = useId()

  const send = (e: FormEvent) => {
    e.preventDefault()
    if (!openHail) return
    if (!reply.trim()) {
      setProblem('Type an answer first.')
      document.getElementById(`${id}-reply`)?.focus()
      return
    }
    answer.mutate(
      { projectId, agentId: agent.id, hailId: openHail.id, answer: reply },
      {
        onSuccess: () => {
          setSent(`Answer sent. ${agent.name} is back ${agentStateLabels.working.toLowerCase()}.`)
          setReply('')
        },
      },
    )
  }

  return (
    <div className="min-w-0 flex-[999_1_560px] rounded-card border border-rule bg-ground p-5">
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="m-0 text-lg font-semibold">
          {agent.name}
          {task && ` · ${task.title}`}
        </h2>
        <Chip tone={look.tone}>
          <Icon name={look.icon} size={12} />
          {openHail ? `${labels.hail} · ${labels.waitingOnYou}` : agentStateLabels[agent.state]}
        </Chip>
      </div>
      {task ? (
        <Link to={`/projects/${projectId}/tasks/${task.id}`} className="mt-1 inline-block font-mono text-[13px]">
          Open {task.key}
        </Link>
      ) : (
        <p className="m-0 mt-2 text-[13px] text-ink-2">
          No current {labels.task.toLowerCase()}. {agent.name} is waiting in the {labels.backlog}.
        </p>
      )}

      <div role="status" aria-live="polite">
        {sent && !openHail && (
          <p className="m-0 mt-4 flex items-center gap-2 text-[13px] text-good">
            <Icon name="check" size={16} />
            {sent}
          </p>
        )}
      </div>

      {openHail && (
        <form onSubmit={send} noValidate className="mt-4 rounded-control border border-approve bg-approve/6 p-3.5">
          <div className="text-[13px] font-medium text-approve-ink">{agent.name} asks</div>
          <p className="m-0 mt-1.5 leading-normal">{openHail.question}</p>
          <div className="mt-3 flex flex-wrap gap-2.5">
            <label htmlFor={`${id}-reply`} className="sr-only">
              Reply to {agent.name}
            </label>
            <TextInput
              id={`${id}-reply`}
              placeholder={`Reply to ${agent.name}`}
              value={reply}
              aria-invalid={!!problem}
              aria-describedby={problem ? `${id}-problem` : undefined}
              onChange={(e) => {
                setReply(e.target.value)
                setProblem(null)
              }}
              className="flex-[1_1_260px] bg-paper"
            />
            <Button type="submit" variant="primary" disabled={answer.isPending} className="px-[18px]">
              {actionLabels.reply}
            </Button>
          </div>
          {(problem || answer.error) && (
            <p id={`${id}-problem`} role="alert" className="m-0 mt-2 text-[13px] text-signal-ink">
              {problem ?? answer.error?.message}
            </p>
          )}
        </form>
      )}

      <h3 className="text-xs font-semibold text-ink-2 m-0 mt-[18px]">Trace</h3>
      {trace.length === 0 ? (
        <p className="m-0 mt-2 text-[13px] text-ink-2">No trace yet.</p>
      ) : (
        <ol className="m-0 mt-2 list-none overflow-x-auto p-0 font-mono text-[13px] leading-[1.8] text-ink">
          {trace.map((s) => (
            <li key={s.id} className="flex gap-4 whitespace-nowrap">
              <time dateTime={s.at} className={s.kind === 'hail' ? 'text-approve-ink' : 'text-ink-2'}>
                {clockTime(s.at)}
              </time>
              <span className="w-16 flex-none">{traceWord[s.kind]}</span>
              <span className="whitespace-normal">{s.text}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}
