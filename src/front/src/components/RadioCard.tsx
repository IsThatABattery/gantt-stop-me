import type { InputHTMLAttributes, ReactNode } from 'react'

interface RadioCardProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: ReactNode
  description?: ReactNode
  /** Shown above the label, e.g. a theme's preview */
  preview?: ReactNode
  /** Kept for older call sites; every radio card looks the same */
  tone?: 'ion' | 'astronaut'
}

/**
 * A radio button laid out as a paper card. Chosen, its edge turns a 2px accent (ink in Light and Dark), beside the
 * filled radio dot, so the choice never rests on colour alone.
 */
export function RadioCard({ label, description, preview, tone: _tone, checked, className = '', ...rest }: RadioCardProps) {
  return (
    <label
      className={`flex cursor-pointer flex-col gap-1.5 rounded-card bg-paper transition-colors duration-150 has-disabled:cursor-not-allowed has-disabled:opacity-60 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-accent ${
        checked ? 'border-2 border-accent p-[13px]' : 'border border-line p-3.5 hover:border-ink'
      } ${className}`}
    >
      {preview}
      <span className="flex items-center gap-2 font-semibold">
        <input type="radio" checked={checked} className="m-0 size-4 flex-none accent-accent" {...rest} />
        {label}
      </span>
      {description && <span className="text-[13px] leading-[1.45] text-ink-2">{description}</span>}
    </label>
  )
}
