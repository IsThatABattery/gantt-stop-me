import type { DocSection } from '../../../data/types'
import { sectionAnchor } from './docText'

/** Text with every [TARGET] placeholder highlighted, so it is easy to spot. */
export function WithTargets({ text }: { text: string }) {
  const parts = text.split(/(\[TARGET\])/)
  return (
    <>
      {parts.map((part, i) =>
        part === '[TARGET]' ? (
          <mark key={i} className="rounded bg-approve/20 px-1 font-mono text-[0.9em] text-approve-ink">
            [TARGET]
            <span className="sr-only"> (placeholder, needs a real value)</span>
          </mark>
        ) : (
          part
        ),
      )}
    </>
  )
}

/** One section of the objective doc: a paragraph or a list. */
export function DocSectionView({ section }: { section: DocSection }) {
  return (
    <section id={sectionAnchor(section.key)} aria-labelledby={`${sectionAnchor(section.key)}-h`} className="scroll-mt-6 border-t border-keyline pt-5">
      <h3 id={`${sectionAnchor(section.key)}-h`} className="mt-0 mb-2 text-base font-semibold">
        {section.title}
      </h3>
      {Array.isArray(section.body) ? (
        <ul className="m-0 list-disc pl-5 leading-[1.7] text-ink">
          {section.body.map((item, i) => (
            <li key={i}>
              <WithTargets text={item} />
            </li>
          ))}
        </ul>
      ) : (
        <p className="m-0 leading-[1.65] text-ink">
          <WithTargets text={section.body} />
        </p>
      )}
    </section>
  )
}
