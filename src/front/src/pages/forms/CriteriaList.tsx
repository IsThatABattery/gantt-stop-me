import { useState } from 'react'
import { Button } from '../../components/controls'
import type { Criterion } from '../../data/types'
import { actionLabels } from '../../labels/labels'

interface CriteriaListProps {
  heading: string
  help: string
  criteria: Criterion[]
  onChange: (criteria: Criterion[]) => void
}

let nextLocal = 1

/** Success or acceptance criteria: add, edit, remove, mark met. */
export function CriteriaList({ heading, help, criteria, onChange }: CriteriaListProps) {
  const [added, setAdded] = useState<string | null>(null)
  const met = criteria.filter((c) => c.met).length
  const update = (id: string, patch: Partial<Criterion>) => onChange(criteria.map((c) => (c.id === id ? { ...c, ...patch } : c)))

  const add = () => {
    const id = `new-c${nextLocal++}`
    setAdded(id)
    onChange([...criteria, { id, text: '', met: false }])
  }

  return (
    <section aria-labelledby="criteria-heading" className="flex flex-col gap-4 rounded-card border border-line bg-surface p-6">
      <div className="flex items-baseline justify-between gap-3">
        <h2 id="criteria-heading" className="m-0 font-heading text-lg font-semibold">
          {heading}
        </h2>
        <span className="font-mono text-xs text-text-muted">
          {met} / {criteria.length} met
        </span>
      </div>
      <p className="m-0 text-[13px] leading-normal text-text-muted-strong">{help}</p>
      {criteria.length > 0 && (
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {criteria.map((c, i) => (
            <li key={c.id} className="flex items-center gap-3 rounded-control border border-line-soft bg-surface-sunken px-3">
              <input
                type="checkbox"
                checked={c.met}
                onChange={(e) => update(c.id, { met: e.target.checked })}
                aria-label={`Met: ${c.text || `criterion ${i + 1}`}`}
                className="size-5 flex-none accent-good"
              />
              <input
                type="text"
                value={c.text}
                placeholder="Describe the criterion"
                aria-label={`Criterion ${i + 1}`}
                // Only the row just added takes focus.
                // oxlint-disable-next-line jsx-a11y/no-autofocus
                autoFocus={c.id === added}
                onChange={(e) => update(c.id, { text: e.target.value })}
                className={`min-h-11 min-w-0 flex-1 border-0 bg-transparent font-[inherit] focus:outline-none ${
                  c.met ? 'text-text-muted-strong line-through' : 'text-text'
                }`}
              />
              <button
                type="button"
                aria-label={`Remove criterion ${i + 1}`}
                onClick={() => onChange(criteria.filter((x) => x.id !== c.id))}
                className="inline-flex size-9 flex-none cursor-pointer items-center justify-center rounded-control border-0 bg-transparent text-text-muted-strong hover:text-text"
              >
                ×
              </button>
            </li>
          ))}
        </ul>
      )}
      <div>
        <Button onClick={add}>{actionLabels.addCriterion}</Button>
      </div>
    </section>
  )
}
