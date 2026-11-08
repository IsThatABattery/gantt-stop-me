import type { TraceStep } from '../../data/types'
import { taskTabLabels, traceKindLabels } from '../../labels/labels'
import { clockTime, traceTone } from './look'

/** The astronaut's step-by-step trace, oldest first (PayloadDetail.dc.html). */
export function Trace({ steps }: { steps: TraceStep[] }) {
  return (
    <section aria-labelledby="trace-heading" className="flex flex-col gap-1 border-t border-line-soft px-6 py-5">
      <h2 id="trace-heading" className="eyebrow m-0 pb-2">
        {taskTabLabels.trace}
      </h2>
      <ol className="m-0 list-none p-0">
        {steps.map((s, i) => (
          <li key={s.id} className="flex gap-4">
            <div className="flex flex-col items-center" aria-hidden="true">
              <span className={`inline-flex size-7 flex-none items-center justify-center rounded-control border font-mono text-xs ${traceTone[s.kind]}`}>
                {i + 1}
              </span>
              {i < steps.length - 1 && <span className="min-h-4 w-px flex-1 bg-line" />}
            </div>
            <div className="min-w-0 pb-5">
              <div className="flex items-baseline gap-2">
                <span className="font-medium">{traceKindLabels[s.kind]}</span>
                <time dateTime={s.at} className="font-mono text-xs text-text-muted">
                  {clockTime(s.at)}
                </time>
              </div>
              <p className="m-0 mt-1 font-mono text-[13px] leading-normal text-text-soft">{s.text}</p>
            </div>
          </li>
        ))}
      </ol>
    </section>
  )
}
