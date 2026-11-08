import { useEffect, useState } from 'react'
import type { Project, Sprint } from '../../data/types'
import { labels } from '../../labels/labels'
import { countdown, isFinalDay, shortDate } from '../../lib/time'
import type { CounterFilter } from './filters'

interface TelemetryBarProps {
  project: Project
  sprint: Sprint | null
  counts: { awaitingApproval: number; blocked: number; agentsActive: number }
  /** The counter currently filtering the board */
  active: CounterFilter | null
  /** Clicking the active counter again clears the filter */
  onToggle: (filter: CounterFilter) => void
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

// Top bar from Main.dc.html. The three counters double as board filters.
export function TelemetryBar({ project, sprint, counts, active, onToggle }: TelemetryBarProps) {
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
        {counters.map((c) => (
          <FilterCounter
            key={c.filter}
            tone={c.tone}
            count={counts[c.count]}
            label={c.label}
            pressed={active === c.filter}
            onClick={() => onToggle(c.filter)}
          />
        ))}
      </div>
    </header>
  )
}

const counters = [
  { filter: 'awaiting-approval', tone: 'warning', count: 'awaitingApproval', label: labels.awaitingApproval },
  { filter: 'blocked', tone: 'critical', count: 'blocked', label: labels.blocked },
  { filter: 'agents-active', tone: 'astronaut', count: 'agentsActive', label: labels.agentsActive },
] as const

const tones = {
  warning: { box: 'border-warning bg-warning/8', on: 'bg-warning/20 ring-2 ring-warning', num: 'text-warning' },
  critical: { box: 'border-critical bg-critical/8', on: 'bg-critical/20 ring-2 ring-critical', num: 'text-critical' },
  astronaut: { box: 'border-astronaut bg-astronaut/8', on: 'bg-astronaut/20 ring-2 ring-astronaut', num: 'text-astronaut' },
}

// components/Counter with a pressed state. Kept here while the shared
// components are frozen; Counter can take `pressed` in a later shared PR.
function FilterCounter({
  tone,
  count,
  label,
  pressed,
  onClick,
}: {
  tone: keyof typeof tones
  count: number
  label: string
  pressed: boolean
  onClick: () => void
}) {
  const t = tones[tone]
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onClick}
      className={`flex min-h-11 cursor-pointer items-center gap-2.5 rounded-control border px-3.5 text-text ${t.box} ${pressed ? t.on : ''}`}
    >
      <span className={`font-mono text-lg ${t.num}`}>{count}</span>
      {label}
    </button>
  )
}
