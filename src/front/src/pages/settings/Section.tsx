import { useId, type ReactNode } from 'react'

/** One settings card with its heading. */
export function Section({ id, title, intro, children }: { id?: string; title: string; intro?: ReactNode; children: ReactNode }) {
  const headingId = useId()
  return (
    <section id={id} aria-labelledby={headingId} className="flex scroll-mt-6 flex-col gap-[18px] rounded-[14px] border border-line bg-surface p-6">
      <div>
        <h2 id={headingId} className="m-0 font-heading text-lg font-semibold">
          {title}
        </h2>
        {intro && <p className="m-0 mt-1.5 leading-normal text-text-muted-strong">{intro}</p>}
      </div>
      {children}
    </section>
  )
}
