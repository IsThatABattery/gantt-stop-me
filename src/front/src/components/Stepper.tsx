import { Link } from 'react-router'
import { Icon } from './Icon'

export interface Step {
  label: string
  /** Where a finished step links back to */
  to?: string
}

interface StepperProps {
  label: string
  steps: Step[]
  /** Index of the current step */
  current: number
}

/** Steps as square marks: the current one solid ink, finished ones checked, the rest hollow. */
export function Stepper({ label, steps, current }: StepperProps) {
  return (
    <ol aria-label={label} className="m-0 flex list-none flex-wrap items-center gap-2.5 p-0 text-[13px]">
      {steps.map((s, i) => {
        const done = i < current
        const now = i === current
        const marker = (
          <span
            aria-hidden="true"
            className={`inline-flex size-6 flex-none items-center justify-center rounded-[2px] border-[1.5px] text-xs font-bold ${
              now ? 'border-ink bg-ink text-on-ink' : done ? 'border-ink bg-paper text-ink' : 'border-line text-ink-2'
            }`}
          >
            {done ? <Icon name="check" size={14} strokeWidth={2.5} /> : i + 1}
          </span>
        )
        const text = (
          <>
            {marker}
            <span>{s.label}</span>
            {done && <span className="sr-only"> (done)</span>}
          </>
        )
        return (
          <li key={s.label} className="flex items-center gap-2.5">
            {i > 0 && <span aria-hidden="true" className="h-[1.5px] w-6 bg-line" />}
            {done && s.to ? (
              <Link to={s.to} className="flex min-h-11 items-center gap-2 font-semibold text-ink no-underline hover:text-ink hover:underline">
                {text}
              </Link>
            ) : (
              <span aria-current={now ? 'step' : undefined} className={`flex items-center gap-2 ${now ? 'font-bold text-ink' : done ? 'font-semibold text-ink' : 'text-ink-2'}`}>
                {text}
              </span>
            )}
          </li>
        )
      })}
    </ol>
  )
}
