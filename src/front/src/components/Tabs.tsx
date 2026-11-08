import { useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'

export interface TabItem {
  id: string
  label: ReactNode
  content: ReactNode
}

interface TabsProps {
  /** Name of the tab list for screen readers */
  label: string
  items: TabItem[]
  /** Controlled selection; omit to let Tabs manage it */
  value?: string
  onChange?: (id: string) => void
}

/** Tab list with arrow-key, Home and End navigation. */
export function Tabs({ label, items, value, onChange }: TabsProps) {
  const [own, setOwn] = useState(items[0]?.id)
  const selected = value ?? own
  const base = useId()
  const refs = useRef<(HTMLButtonElement | null)[]>([])

  const select = (id: string) => {
    setOwn(id)
    onChange?.(id)
  }

  const onKey = (e: KeyboardEvent, i: number) => {
    const last = items.length - 1
    const next = e.key === 'ArrowRight' ? (i === last ? 0 : i + 1) : e.key === 'ArrowLeft' ? (i === 0 ? last : i - 1) : e.key === 'Home' ? 0 : e.key === 'End' ? last : -1
    if (next < 0) return
    e.preventDefault()
    select(items[next].id)
    refs.current[next]?.focus()
  }

  return (
    <div>
      <div role="tablist" aria-label={label} className="flex gap-1 overflow-x-auto border-b border-line px-1">
        {items.map((t, i) => {
          const active = t.id === selected
          return (
            <button
              key={t.id}
              ref={(el) => {
                refs.current[i] = el
              }}
              role="tab"
              type="button"
              id={`${base}-tab-${t.id}`}
              aria-selected={active}
              aria-controls={`${base}-panel-${t.id}`}
              tabIndex={active ? 0 : -1}
              onClick={() => select(t.id)}
              onKeyDown={(e) => onKey(e, i)}
              className={`-mb-px min-h-11 cursor-pointer border-0 border-b-2 bg-transparent px-3 font-[inherit] whitespace-nowrap ${
                active ? 'border-ion font-medium text-text' : 'border-transparent text-text-muted-strong hover:text-text'
              }`}
            >
              {t.label}
            </button>
          )
        })}
      </div>
      {items.map((t) =>
        t.id === selected ? (
          <div key={t.id} role="tabpanel" id={`${base}-panel-${t.id}`} aria-labelledby={`${base}-tab-${t.id}`} tabIndex={0} className="outline-none">
            {t.content}
          </div>
        ) : null,
      )}
    </div>
  )
}
