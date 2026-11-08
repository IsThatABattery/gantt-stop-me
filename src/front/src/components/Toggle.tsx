interface ToggleProps {
  id?: string
  checked: boolean
  onChange: (checked: boolean) => void
  /** Accessible name when there is no visible label pointing at it */
  label?: string
  disabled?: boolean
}

/** On/off switch. */
export function Toggle({ id, checked, onChange, label, disabled }: ToggleProps) {
  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-7 w-12 flex-none cursor-pointer items-center rounded-control border p-1 disabled:cursor-not-allowed disabled:opacity-50 ${
        checked ? 'border-ion bg-ion/25' : 'border-line bg-surface-sunken'
      }`}
    >
      <span className={`size-5 rounded-control transition-transform ${checked ? 'translate-x-5 bg-ion' : 'translate-x-0 bg-text-muted'}`} />
      <span className="sr-only">{checked ? 'On' : 'Off'}</span>
    </button>
  )
}
