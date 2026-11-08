import type { Tone } from './Chip'

const tones: Record<Extract<Tone, 'warning' | 'critical' | 'astronaut'>, { box: string; num: string }> = {
  warning: { box: 'border-warning bg-warning/8', num: 'text-warning' },
  critical: { box: 'border-critical bg-critical/8', num: 'text-critical' },
  astronaut: { box: 'border-astronaut bg-astronaut/8', num: 'text-astronaut' },
}

interface CounterProps {
  tone: keyof typeof tones
  count: number
  label: string
  onClick?: () => void
}

/** Telemetry counter: a count plus its plain word. */
export function Counter({ tone, count, label, onClick }: CounterProps) {
  const t = tones[tone]
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex min-h-11 cursor-pointer items-center gap-2.5 rounded-control border px-3.5 text-text ${t.box}`}
    >
      <span className={`font-mono text-lg ${t.num}`}>{count}</span>
      {label}
    </button>
  )
}
