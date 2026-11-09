import { useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'

interface TabItem {
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

/** Blocky tabs: the selected one is a solid ink tab on a 2px ink rule. Arrow keys, Home and End move between them. */
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
      <div role="tablist" aria-label={label} className="flex gap-1 overflow-x-auto border-b-2 border-ink">
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
              className={`min-h-11 cursor-pointer rounded-t-[3px] border-0 px-4 font-[inherit] text-sm font-semibold whitespace-nowrap transition-colors duration-150 ${
                active ? 'bg-ink text-on-ink' : 'bg-transparent text-ink-2 hover:bg-ink/8 hover:text-ink active:bg-ink/15'
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
