interface ProgressBarProps {
  label: string
  value: number
  max: number
}

export function ProgressBar({ label, value, max }: ProgressBarProps) {
  const pct = max > 0 ? Math.round((value / max) * 100) : 0
  return (
    <div>
      <div className="flex justify-between pb-2 text-xs text-text-muted">
        <span>{label}</span>
        <span className="font-mono">
          {value} / {max}
        </span>
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={value}
        className="h-2 overflow-hidden rounded-control bg-line-soft"
      >
        <div className="h-2 bg-good" style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}
