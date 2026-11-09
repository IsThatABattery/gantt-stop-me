import type { TraceStep } from '../../data/types'
import { taskTabLabels, traceKindLabels } from '../../labels/labels'
import { clockTime, traceTone } from './look'

/** An agent's step-by-step trace, oldest first (PayloadDetail.dc.html). */
export function Trace({ steps }: { steps: TraceStep[] }) {
  return (
    <section aria-labelledby="trace-heading" className="flex flex-col gap-1 border-t border-keyline px-6 py-5">
      <h2 id="trace-heading" className="text-xs font-semibold text-ink-2 m-0 pb-2">
        {taskTabLabels.trace}
      </h2>
      <ol className="m-0 list-none p-0">
        {steps.map((s, i) => (
          <li key={s.id} className="flex gap-3.5">
            <div className="flex flex-col items-center" aria-hidden="true">
              <span className={`inline-flex size-[26px] flex-none items-center justify-center rounded-[2px] border-[1.5px] font-mono text-xs ${traceTone[s.kind]}`}>
                {i + 1}
              </span>
              {i < steps.length - 1 && <span className="min-h-4 w-px flex-1 bg-rule" />}
            </div>
            <div className="min-w-0 pb-[18px]">
              <div className="flex items-baseline gap-2">
                <span className="font-medium">{traceKindLabels[s.kind]}</span>
                <time dateTime={s.at} className="font-mono text-xs text-ink-2">
                  {clockTime(s.at)}
                </time>
              </div>
              <p className="m-0 mt-1 font-mono text-[13px] leading-normal text-ink">{s.text}</p>
            </div>
          </li>
        ))}
      </ol>
    </section>
  )
}
