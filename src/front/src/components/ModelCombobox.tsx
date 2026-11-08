import { useId, useState, type KeyboardEvent } from 'react'
import { cx } from '../lib/cx'
import { Icon } from './Icon'

interface ModelComboboxProps {
  id: string
  options: string[]
  /** The chosen models; at most one without `multiple` */
  selected: string[]
  onChange: (selected: string[]) => void
  /** Tick several models, with Select all; otherwise picking one closes the list */
  multiple?: boolean
  placeholder?: string
  /** The most Select all ticks: the API's cap on the list */
  max?: number
  disabled?: boolean
  /** Reads quieter, for a field secondary to the one before it */
  quiet?: boolean
  'aria-label'?: string
  'aria-invalid'?: boolean
  'aria-describedby'?: string
}

/** A search box over a list of models, in the site's theme. Type to filter, arrows and Enter to pick. */
export function ModelCombobox({ id, options, selected, onChange, multiple, placeholder, max, disabled, quiet, ...aria }: ModelComboboxProps) {
  const listId = useId()
  // null while not searching: the box then shows the choice, or for `multiple` stays empty.
  const [query, setQuery] = useState<string | null>(null)
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const shown = query ? options.filter((m) => m.toLowerCase().includes(query.toLowerCase())) : options
  const allShownChosen = shown.length > 0 && (shown.every((m) => selected.includes(m)) || (max !== undefined && selected.length >= max))

  const close = () => {
    setOpen(false)
    setQuery(null)
  }
  const pick = (m: string) => {
    if (!multiple) {
      onChange([m])
      close()
    } else onChange(selected.includes(m) ? selected.filter((x) => x !== m) : [...selected, m])
  }
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      setOpen(true)
      setActive((i) => Math.max(0, Math.min(shown.length - 1, i + (e.key === 'ArrowDown' ? 1 : -1))))
    } else if (e.key === 'Enter' && open) {
      e.preventDefault() // don't submit the form
      if (shown[active]) pick(shown[active])
    } else if (e.key === 'Escape' && open) {
      e.stopPropagation() // close the list, not the dialog around it
      close()
    }
  }

  return (
    <div
      className="relative"
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget)) close()
      }}
    >
      <input
        id={id}
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={open && shown[active] ? `${listId}-${active}` : undefined}
        autoComplete="off"
        spellCheck={false}
        disabled={disabled}
        value={query ?? (multiple ? '' : (selected[0] ?? ''))}
        placeholder={multiple && selected.length ? `${selected.length} chosen. Search to add more` : placeholder}
        onFocus={() => setOpen(true)}
        onClick={() => setOpen(true)}
        onChange={(e) => {
          setQuery(e.target.value)
          setActive(0)
          setOpen(true)
        }}
        onKeyDown={onKeyDown}
        className={cx(
          'box-border min-h-[46px] w-full rounded-control border bg-surface-sunken px-3.5 pr-9 font-[inherit] placeholder:text-text-muted focus:border-ion focus:outline-none disabled:cursor-not-allowed disabled:opacity-60',
          quiet ? 'border-line-soft bg-transparent text-text-muted-strong' : 'border-line text-text',
        )}
        {...aria}
      />
      <span className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-xs text-text-muted" aria-hidden="true">
        ▾
      </span>
      {open && !disabled && (
        <div className="absolute z-30 mt-1 w-full overflow-hidden rounded-control border border-line bg-surface shadow-lg">
          {multiple && shown.length > 0 && (
            <div className="flex items-center justify-between border-b border-line-soft px-3 py-1.5 text-xs text-text-muted">
              <span>
                {selected.length} of {options.length} chosen{max !== undefined && options.length > max && `, up to ${max}`}
              </span>
              <button
                type="button"
                className="cursor-pointer border-0 bg-transparent p-1 font-[inherit] text-xs text-ion hover:text-ion-hover"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => onChange(allShownChosen ? selected.filter((m) => !shown.includes(m)) : [...selected, ...shown.filter((m) => !selected.includes(m))].slice(0, max))}
              >
                {allShownChosen ? 'Clear all' : 'Select all'}
              </button>
            </div>
          )}
          <ul id={listId} role="listbox" aria-multiselectable={multiple || undefined} className="m-0 max-h-64 list-none overflow-auto p-1">
            {shown.length === 0 && <li className="px-3 py-2 text-[13px] text-text-muted">No matches.</li>}
            {shown.map((m, i) => {
              const chosen = selected.includes(m)
              return (
                <li
                  key={m}
                  id={`${listId}-${i}`}
                  role="option"
                  aria-selected={chosen}
                  onMouseDown={(e) => e.preventDefault()} // keep focus in the box
                  onMouseEnter={() => setActive(i)}
                  onClick={() => pick(m)}
                  className={`flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-2 font-mono text-[13px] ${i === active ? 'bg-surface-sunken text-text' : 'text-text-soft'}`}
                >
                  {multiple ? (
                    <span
                      className={`flex size-4 flex-none items-center justify-center rounded border ${chosen ? 'border-ion bg-ion text-on-ion' : 'border-line-strong'}`}
                    >
                      {chosen && <Icon name="check" size={12} />}
                    </span>
                  ) : (
                    <span className="flex w-4 flex-none justify-center text-ion">{chosen && <Icon name="check" size={14} />}</span>
                  )}
                  <span className="min-w-0 wrap-anywhere">{m}</span>
                </li>
              )
            })}
          </ul>
        </div>
      )}
    </div>
  )
}
