import { Icon } from '../../components/Icon'
import type { Project } from '../../data/api/types'
import { labels } from '../../labels/labels'
import type { CounterFilter } from './filters'

interface TelemetryBarProps {
  project: Project
  counts: Record<CounterFilter, number>
  /** The counter currently filtering the board */
  active: CounterFilter | null
  /** Clicking the active counter again clears the filter */
  onToggle: (filter: CounterFilter) => void
}

// Top bar from Main.dc.html: the repository, then three counters that double
// as board filters. The launch-window countdown and burndown wait on sprints
// in the API (docs/implementation/api-gaps.md).
export function TelemetryBar({ project, counts, active, onToggle }: TelemetryBarProps) {
  const repo = `${project.owner}/${project.name}`
  return (
    <header className="flex flex-wrap items-center gap-6 border-b border-line-soft bg-header px-7 py-5">
      <div className="min-w-0 flex-[1_1_320px]">
        <h1 className="m-0 font-heading text-2xl font-semibold wrap-anywhere">{project.name}</h1>
        <a
          href={`https://github.com/${repo}`}
          target="_blank"
          rel="noreferrer"
          className="mt-1 inline-flex max-w-full min-h-11 items-center gap-1.5 text-[13px] text-text-muted-strong"
        >
          <Icon name="github" size={14} className="shrink-0" />
          <span className="min-w-0 wrap-anywhere">{repo}</span>
        </a>
      </div>

      <div className="flex flex-wrap gap-2.5">
        {counters.map((c) => (
          <FilterCounter
            key={c.filter}
            tone={c.tone}
            count={counts[c.filter]}
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
  { filter: 'awaiting-approval', tone: 'warning', label: labels.awaitingApproval },
  { filter: 'needs-attention', tone: 'critical', label: labels.blocked },
  { filter: 'tests-running', tone: 'ion', label: labels.testsRunning },
] as const

const tones = {
  warning: { box: 'border-warning bg-warning/8', on: 'bg-warning/20 ring-2 ring-warning', num: 'text-warning' },
  critical: { box: 'border-critical bg-critical/8', on: 'bg-critical/20 ring-2 ring-critical', num: 'text-critical' },
  ion: { box: 'border-ion bg-ion/8', on: 'bg-ion/20 ring-2 ring-ion', num: 'text-ion' },
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
