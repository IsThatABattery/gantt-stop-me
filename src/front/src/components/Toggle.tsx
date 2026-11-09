interface ToggleProps {
  id?: string
  checked: boolean
  onChange: (checked: boolean) => void
  /** Accessible name when there is no visible label pointing at it */
  label?: string
  disabled?: boolean
}

/** On/off switch: a square knob in a 1.5px ink track, filled ink and moved right when on. */
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
      className={`relative inline-flex h-7 w-12 flex-none cursor-pointer items-center rounded-control border-[1.5px] border-ink p-[3px] transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-45 ${
        checked ? 'bg-ink hover:border-ink-hover hover:bg-ink-hover' : 'bg-paper hover:bg-ink/8'
      }`}
    >
      <span
        className={`size-[19px] rounded-[2px] transition-transform duration-150 ease-out-quint ${checked ? 'translate-x-5 bg-on-ink' : 'translate-x-0 bg-ink-2'}`}
      />
      <span className="sr-only">{checked ? 'On' : 'Off'}</span>
    </button>
  )
}
