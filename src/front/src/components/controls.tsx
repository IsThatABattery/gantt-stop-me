import { cx } from '../lib/cx'
import { buttonClass, type ButtonVariant } from './buttonClass'
import { Icon, type IconName } from './Icon'
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  /** Working on it: the button is disabled and says so to screen readers. Pair it with a word, e.g. "Saving…". */
  busy?: boolean
}

export function Button({ variant = 'secondary', className = '', type = 'button', busy, disabled, ...rest }: ButtonProps) {
  return <button type={type} className={buttonClass(variant, className)} disabled={disabled || busy} aria-busy={busy || undefined} {...rest} />
}

/**
 * The square mark that leads a primary action, as on the board's New task button: a paper square holding the icon,
 * set inside the ink fill. Use it inside buttonClass('primary', 'min-h-[52px] pl-2 pr-[18px]').
 */
export function ButtonMark({ icon }: { icon: IconName }) {
  return (
    <span aria-hidden="true" className="grid size-9 flex-none place-items-center rounded-[2px] bg-paper text-ink">
      <Icon name={icon} size={16} />
    </span>
  )
}

/** Paper, a 1px line edge, a 2px accent edge when focused, and the signal edge when aria-invalid. */
const fieldControl =
  'min-h-[46px] box-border rounded-control border border-line bg-paper text-ink font-[inherit] text-sm placeholder:text-ink-2 transition-colors duration-150 hover:border-ink focus:border-accent focus:outline-1 focus:outline-offset-0 focus:outline-accent aria-[invalid=true]:border-signal aria-[invalid=true]:outline-signal disabled:cursor-not-allowed disabled:border-rule disabled:bg-block disabled:text-ink-2'

export function TextInput({ className = '', ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cx(`${fieldControl} w-full px-3.5`, className)} {...rest} />
}

export function Select({ className = '', ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={cx(`${fieldControl} cursor-pointer px-3`, className)} {...rest} />
}

export function Textarea({ className = '', ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={cx(`${fieldControl} w-full px-3.5 py-3 leading-normal`, className)} rows={4} {...rest} />
}

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: ReactNode
}

/** A square box with a 1.5px ink edge, filled ink with a check when on. */
export function Checkbox({ label, className = '', ...rest }: CheckboxProps) {
  return (
    <label className={`flex min-h-11 cursor-pointer items-center gap-2.5 text-sm text-ink has-disabled:cursor-not-allowed has-disabled:text-ink-2 ${className}`}>
      <span className="relative inline-flex size-[18px] flex-none">
        <input
          type="checkbox"
          className="peer m-0 size-[18px] cursor-[inherit] appearance-none rounded-[2px] border-[1.5px] border-ink bg-paper transition-colors duration-150 checked:bg-ink hover:bg-ink/8 checked:hover:bg-ink-hover disabled:border-line disabled:bg-block"
          {...rest}
        />
        <Icon
          name="check"
          size={14}
          strokeWidth={3}
          className="pointer-events-none absolute top-[2px] left-[2px] hidden text-on-ink peer-checked:block"
        />
      </span>
      {label}
    </label>
  )
}

interface FieldProps {
  id: string
  label: ReactNode
  /** Extra content on the label row, e.g. a "Forgot password?" link */
  aside?: ReactNode
  children: ReactNode
}

export function Field({ id, label, aside, children }: FieldProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-baseline justify-between gap-3">
        <label htmlFor={id} className="text-[13px] font-semibold text-ink-2">
          {label}
        </label>
        {aside}
      </div>
      {children}
    </div>
  )
}

/** A field's problem under it, in the signal ink. Renders nothing when there is none. */
export function FieldError({ id, children }: { id: string; children?: ReactNode }) {
  if (!children) return null
  return (
    <span id={id} className="text-[13px] text-signal-ink">
      {children}
    </span>
  )
}
