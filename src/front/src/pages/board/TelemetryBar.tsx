import { useEffect, useState } from 'react'
import { Counter } from '../../components/Counter'
import type { Project, Sprint } from '../../data/types'
import { labels } from '../../labels/labels'
import { countdown, isFinalDay, shortDate } from '../../lib/time'

interface TelemetryBarProps {
  project: Project
  sprint: Sprint | null
  counts: { awaitingApproval: number; blocked: number; agentsActive: number }
}

/** Ticks once a minute so the countdown stays current. */
function useNow() {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60_000)
    return () => clearInterval(id)
  }, [])
  return now
}

// Top bar from Main.dc.html. Counters are inert until the Phase 2 board.
export function TelemetryBar({ project, sprint, counts }: TelemetryBarProps) {
  const now = useNow()
  const finalDay = sprint ? isFinalDay(sprint.end, now) : false

  return (
    <header className="flex flex-wrap items-center gap-6 border-b border-line-soft bg-header px-7 py-5">
      <div className="min-w-0 flex-[1_1_320px]">
        <div className="eyebrow">
          {labels.project} · {project.name}
          {sprint && ` · ${labels.sprintName(sprint.number)}`}
        </div>
        <h1 className="mt-1.5 mb-0 font-heading text-2xl font-semibold">{sprint ? sprint.goal : project.name}</h1>
      </div>

      <div className="flex items-center gap-3 rounded-control border border-line bg-surface px-3.5 py-2">
        {sprint ? (
          <>
            <span className="text-xs text-text-muted">{labels.tMinus}</span>
            <span
              className={`font-mono text-xl font-medium ${finalDay ? 'text-warning' : ''}`}
              aria-label={`${countdown(sprint.end, now)} left in ${labels.sprintName(sprint.number).toLowerCase()}`}
            >
              {countdown(sprint.end, now)}
            </span>
            {/* Static sample burndown until sprint data exists. */}
            <svg width="96" height="32" viewBox="0 0 96 32" role="img" aria-label="Burndown trending slightly behind ideal">
              <path d="M2 4L94 28" stroke="var(--color-line)" strokeWidth="1.5" strokeDasharray="3 3" fill="none" />
              <path d="M2 4L18 7L34 9L50 15L66 17L78 20" stroke="var(--color-ion)" strokeWidth="2" fill="none" />
            </svg>
          </>
        ) : (
          <span className="text-[13px] text-text-muted-strong">
            {labels.betweenSprints}
            {project.nextSprintStart && (
              <span className="ml-2 font-mono text-text-muted">{labels.nextSprint(shortDate(project.nextSprintStart))}</span>
            )}
          </span>
        )}
      </div>

      <div className="flex flex-wrap gap-2.5">
        <Counter tone="warning" count={counts.awaitingApproval} label={labels.awaitingApproval} />
        <Counter tone="critical" count={counts.blocked} label={labels.blocked} />
        <Counter tone="astronaut" count={counts.agentsActive} label={labels.agentsActive} />
      </div>
    </header>
  )
}
