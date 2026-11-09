import { AgentAvatar, AutonomyRings } from '../../components/Avatar'
import { Chip } from '../../components/Chip'
import { Icon } from '../../components/Icon'
import type { AgentRow } from '../../data/views/agents'
import { agentStateLabels, autonomyLabels, labels } from '../../labels/labels'
import { stateLook, timeOnTask } from './look'

interface RosterTileProps {
  row: AgentRow
  selected: boolean
  onSelect: () => void
}

/** One astronaut in the roster (ProbeConsole.dc.html). Amber border when it has an open hail. */
export function RosterTile({ row, selected, onSelect }: RosterTileProps) {
  const { agent, task, openHail } = row
  const look = stateLook[agent.state]
  const border = openHail ? 'border-approve' : selected ? 'border-accent' : 'border-rule'

  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onSelect}
      className={`flex cursor-pointer flex-col gap-3 rounded-card border bg-paper p-4 text-left font-[inherit] text-ink ${border} ${
        selected ? 'outline-2 outline-offset-2 outline-accent' : 'hover:border-line'
      }`}
    >
      <span className="flex w-full flex-wrap items-center gap-3">
        <AgentAvatar agent={agent} active={agent.state === 'working' && !agent.paused} hail={!!openHail} />
        <span className="min-w-0">
          <span className="block text-base font-semibold whitespace-nowrap">{agent.name}</span>
          <span className="block text-xs text-ink-2">{agent.role}</span>
        </span>
        <span className="ml-auto flex flex-col items-end gap-1">
          <Chip tone={look.tone}>
            <Icon name={look.icon} size={12} />
            {agentStateLabels[agent.state]}
          </Chip>
          {agent.paused && (
            <Chip tone="neutral">
              <Icon name="pause" size={12} />
              Paused
            </Chip>
          )}
        </span>
      </span>
      <span className="text-[13px] leading-[1.45]">
        {task ? (
          <>
            <span className="font-mono text-ink-2">{task.key}</span> {task.title}
          </>
        ) : (
          <span className="text-ink-2">No current {labels.task.toLowerCase()}</span>
        )}
      </span>
      <span className="flex w-full items-center justify-between text-xs text-ink-2">
        <span className="inline-flex items-center gap-1.5">
          Autonomy: {autonomyLabels[agent.autonomy]} <AutonomyRings level={agent.autonomy} size={12} />
        </span>
        <span className="font-mono">
          <span className="sr-only">Time on task: </span>
          {task ? timeOnTask(agent.timeOnTaskMinutes) : '—'}
        </span>
      </span>
    </button>
  )
}
