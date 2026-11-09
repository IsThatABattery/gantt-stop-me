import { useState, type KeyboardEvent } from 'react'
import { Icon } from '../../components/Icon'

interface TagInputProps {
  id: string
  label: string
  tags: string[]
  onChange: (tags: string[]) => void
}

/** Tag chips: type and press Enter to add, the cross on a chip to remove it. */
export function TagInput({ id, label, tags, onChange }: TagInputProps) {
  const [draft, setDraft] = useState('')

  const add = () => {
    const tag = draft.trim()
    if (tag && !tags.includes(tag)) onChange([...tags, tag])
    setDraft('')
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      add()
    } else if (e.key === 'Backspace' && !draft && tags.length > 0) {
      onChange(tags.slice(0, -1))
    }
  }

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-[13px] font-semibold text-ink-2">
        {label}
      </label>
      <div className="flex min-h-[46px] flex-wrap items-center gap-1.5 rounded-control border border-line bg-paper px-2 py-1.5 transition-colors duration-150 focus-within:border-accent focus-within:outline-1 focus-within:outline-accent hover:border-ink">
        {tags.map((tag) => (
          <span key={tag} className="inline-flex h-7 items-center gap-0.5 rounded-[2px] bg-block pl-2 text-xs font-semibold text-ink">
            {tag}
            <button
              type="button"
              aria-label={`Remove tag ${tag}`}
              onClick={() => onChange(tags.filter((t) => t !== tag))}
              className="inline-flex size-7 cursor-pointer items-center justify-center rounded-[2px] border-0 bg-transparent text-ink-2 hover:bg-block-hover hover:text-ink"
            >
              <Icon name="close" size={14} />
            </button>
          </span>
        ))}
        <input
          id={id}
          type="text"
          value={draft}
          placeholder={tags.length ? 'Add a tag' : 'Add a tag and press Enter'}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKeyDown}
          onBlur={add}
          className="min-h-8 min-w-[120px] flex-1 border-0 bg-transparent px-1.5 font-[inherit] text-sm text-ink placeholder:text-ink-2 focus:outline-none"
        />
      </div>
    </div>
  )
}
