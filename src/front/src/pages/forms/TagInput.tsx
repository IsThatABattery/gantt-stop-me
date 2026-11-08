import { useState, type KeyboardEvent } from 'react'

interface TagInputProps {
  id: string
  label: string
  tags: string[]
  onChange: (tags: string[]) => void
}

/** Plain tag chips: type and press Enter to add, × to remove. Not themed. */
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
      <label htmlFor={id} className="text-[13px] text-text-soft">
        {label}
      </label>
      <div className="flex min-h-[46px] flex-wrap items-center gap-2 rounded-control border border-line bg-surface-sunken px-2.5 py-1.5 focus-within:border-ion">
        {tags.map((tag) => (
          <span key={tag} className="inline-flex items-center gap-1 rounded-lg border border-line-strong py-0.5 pr-0.5 pl-2.5 text-[13px]">
            {tag}
            <button
              type="button"
              aria-label={`Remove tag ${tag}`}
              onClick={() => onChange(tags.filter((t) => t !== tag))}
              className="inline-flex size-6 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent text-text-muted-strong hover:text-text"
            >
              ×
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
          className="min-h-9 min-w-[120px] flex-1 border-0 bg-transparent px-1 font-[inherit] text-text placeholder:text-text-muted focus:outline-none"
        />
      </div>
    </div>
  )
}
