import type { InputHTMLAttributes, ReactNode } from 'react'

interface RadioCardProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: ReactNode
  description?: ReactNode
  /** Shown above the label, e.g. a theme's preview */
  preview?: ReactNode
  /** astronaut (violet) for autonomy choices, ion otherwise */
  tone?: 'ion' | 'astronaut'
}

/** A radio button laid out as a card, from the Settings and feedback mockups. */
export function RadioCard({ label, description, preview, tone = 'ion', checked, className = '', ...rest }: RadioCardProps) {
  const on = tone === 'astronaut' ? 'border-astronaut bg-astronaut/10' : 'border-ion bg-ion/8'
  return (
    <label
      className={`flex cursor-pointer flex-col gap-1.5 rounded-control border p-3.5 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-ion ${
        checked ? on : 'border-line'
      } ${className}`}
    >
      {preview}
      <span className="flex items-center gap-2 font-semibold">
        <input type="radio" checked={checked} className={`size-4 ${tone === 'astronaut' ? 'accent-astronaut' : 'accent-ion'}`} {...rest} />
        {label}
      </span>
      {description && <span className="text-[13px] leading-[1.45] text-text-muted-strong">{description}</span>}
    </label>
  )
}
