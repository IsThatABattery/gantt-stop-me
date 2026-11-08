import { useState } from 'react'
import { Link } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { buttonClass } from '../../components/buttonClass'
import { Eyebrow } from '../../components/Eyebrow'
import { StatusMessage } from '../../components/StatusMessage'
import { useUser } from '../../data/core'
import { useSession } from '../../data/session'
import { useAgents, useAutonomyPolicy } from '../../data/views/agents'
import { actionLabels, labels } from '../../labels/labels'
import { AgentPanel } from './AgentPanel'
import { Controls } from './Controls'
import { RosterTile } from './RosterTile'

// Astronaut console, from ProbeConsole.dc.html (AGENT_BRIEF §6).
export function AgentsPage() {
  const project = useProjectContext()
  const { data: rows, isPending, error } = useAgents(project.id)
  const { data: policy } = useAutonomyPolicy(project.id)
  const session = useSession()
  const { data: me } = useUser(session?.userId)
  const [picked, setPicked] = useState<string | null>(null)

  const active = rows?.filter((r) => r.task && r.agent.state !== 'idle').length ?? 0
  const needsYou = rows?.filter((r) => r.openHail).length ?? 0
  // Start on the astronaut waiting on a hail, else the first one, and stay
  // there once its hail is answered.
  const initial = rows?.find((r) => r.openHail) ?? rows?.[0]
  if (picked === null && initial) setPicked(initial.agent.id)
  const selected = rows?.find((r) => r.agent.id === picked) ?? initial
  const board = `/projects/${project.id}`

  return (
    <div className="flex flex-col gap-6 px-8 py-7">
      <header className="flex flex-wrap items-end gap-4">
        <div className="flex-[1_1_320px]">
          <Eyebrow>
            {labels.agents} · {active} active · {needsYou} {labels.needsYou.toLowerCase()}
          </Eyebrow>
          <h1 className="mt-2 mb-0 font-heading text-[26px] font-semibold">{labels.agent} console</h1>
        </div>
        <Link to={board} className={buttonClass('secondary')}>
          {actionLabels.backToBoard}
        </Link>
      </header>

      {isPending ? (
        <StatusMessage title={`Loading ${labels.agents.toLowerCase()}…`} />
      ) : error ? (
        <StatusMessage title="Something went wrong" body={error.message} />
      ) : rows.length === 0 || !selected ? (
        <StatusMessage
          title={`No ${labels.agents.toLowerCase()} on this ${labels.project.toLowerCase()} yet`}
          body={`Assign a ${labels.task.toLowerCase()} to an ${labels.agent.toLowerCase()} and it shows up here.`}
        >
          <Link to={board} className={buttonClass('secondary')}>
            {actionLabels.backToBoard}
          </Link>
        </StatusMessage>
      ) : (
        <>
          <div role="group" aria-label={`${labels.agents} roster`} className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-4">
            {rows.map((r) => (
              <RosterTile key={r.agent.id} row={r} selected={r.agent.id === selected.agent.id} onSelect={() => setPicked(r.agent.id)} />
            ))}
          </div>
          <section aria-label={`Selected ${labels.agent.toLowerCase()}: ${selected.agent.name}`} className="flex flex-wrap gap-4">
            <AgentPanel key={selected.agent.id} row={selected} projectId={project.id} />
            <Controls key={`c-${selected.agent.id}`} row={selected} projectId={project.id} me={me} freeFlightGrantedBy={policy?.freeFlightGrantedBy} />
          </section>
        </>
      )}
    </div>
  )
}
