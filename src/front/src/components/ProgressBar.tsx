interface ProgressBarProps {
  label: string
  value: number
  max: number
}

/** A label and "3 / 5" over a square-ended ink bar in a block track. */
export function ProgressBar({ label, value, max }: ProgressBarProps) {
  const pct = max > 0 ? Math.round((value / max) * 100) : 0
  return (
    <div>
      <div className="flex justify-between pb-1.5 text-xs text-ink-2">
        <span className="font-semibold">{label}</span>
        <span>
          {value} / {max}
        </span>
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={value}
        className="h-2 overflow-hidden rounded-[2px] bg-block"
      >
        <div className="h-2 bg-ink transition-[width] duration-200 ease-out-quint" style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}
