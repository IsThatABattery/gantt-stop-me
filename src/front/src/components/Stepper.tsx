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

/** Numbered steps from the planner mockups; finished steps show a check. */
export function Stepper({ label, steps, current }: StepperProps) {
  return (
    <ol aria-label={label} className="m-0 flex list-none flex-wrap items-center gap-2.5 p-0 text-[13px]">
      {steps.map((s, i) => {
        const done = i < current
        const now = i === current
        const marker = (
          <span
            aria-hidden="true"
            className={`inline-flex size-6 items-center justify-center rounded-full border text-xs ${
              now ? 'border-ion bg-ion/12 font-semibold text-text' : done ? 'border-good text-good' : 'border-line text-text-muted'
            }`}
          >
            {done ? <Icon name="check" size={13} strokeWidth={2.25} /> : i + 1}
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
            {i > 0 && <span aria-hidden="true" className="h-px w-7 bg-line" />}
            {done && s.to ? (
              <Link to={s.to} className="flex min-h-11 items-center gap-2 text-text-soft no-underline hover:text-text">
                {text}
              </Link>
            ) : (
              <span aria-current={now ? 'step' : undefined} className={`flex items-center gap-2 ${now ? 'font-medium text-text' : done ? 'text-text-soft' : 'text-text-muted'}`}>
                {text}
              </span>
            )}
          </li>
        )
      })}
    </ol>
  )
}
