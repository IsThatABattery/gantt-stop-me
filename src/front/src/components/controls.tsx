import { cx } from '../lib/cx'
import { buttonClass, type ButtonVariant } from './buttonClass'
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
}

export function Button({ variant = 'secondary', className = '', type = 'button', ...rest }: ButtonProps) {
  return <button type={type} className={buttonClass(variant, className)} {...rest} />
}

const fieldControl =
  'min-h-[46px] box-border rounded-control border border-line bg-surface-sunken text-text font-[inherit] placeholder:text-text-muted focus:border-ion focus:outline-none'

export function TextInput({ className = '', ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cx(`${fieldControl} w-full px-3.5`, className)} {...rest} />
}

export function Select({ className = '', ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={cx(`${fieldControl} px-3`, className)} {...rest} />
}

export function Textarea({ className = '', ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={cx(`${fieldControl} w-full px-3.5 py-3 leading-normal`, className)} rows={4} {...rest} />
}

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: ReactNode
}

export function Checkbox({ label, className = '', ...rest }: CheckboxProps) {
  return (
    <label className={`flex min-h-11 cursor-pointer items-center gap-2.5 text-sm text-text-soft ${className}`}>
      <input type="checkbox" className="size-[18px] accent-ion" {...rest} />
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
      <div className="flex items-baseline justify-between">
        <label htmlFor={id} className="text-[13px] text-text-soft">
          {label}
        </label>
        {aside}
      </div>
      {children}
    </div>
  )
}
