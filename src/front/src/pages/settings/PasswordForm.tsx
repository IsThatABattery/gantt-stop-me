import { useId, useState, type FormEvent } from 'react'
import { Button, Field, TextInput } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { passwordProblems, useChangePassword, type PasswordInput } from '../../data/views/settings'
import { settingsLabels } from '../../labels/labels'

const empty: PasswordInput = { current: '', next: '', confirm: '' }

const fields: { key: keyof PasswordInput; label: string; autoComplete: string; hint?: string }[] = [
  { key: 'current', label: 'Current password', autoComplete: 'current-password' },
  { key: 'next', label: 'New password', autoComplete: 'new-password', hint: '8 or more characters.' },
  { key: 'confirm', label: 'Confirm new password', autoComplete: 'new-password' },
]

/** Mock password change: checks the fields, never a real password. Separate from Save changes. */
export function PasswordForm() {
  const [value, setValue] = useState(empty)
  const [errors, setErrors] = useState<Partial<Record<keyof PasswordInput, string>>>({})
  const [done, setDone] = useState(false)
  const change = useChangePassword()
  const base = useId()

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const problems = passwordProblems(value)
    setErrors(problems)
    setDone(false)
    const first = fields.find((f) => problems[f.key])
    if (first) {
      document.getElementById(`${base}-${first.key}`)?.focus()
      return
    }
    change.mutate(value, {
      onSuccess: () => {
        setValue(empty)
        setDone(true)
      },
    })
  }

  return (
    <form onSubmit={submit} noValidate aria-labelledby={`${base}-heading`} className="flex flex-col gap-4 border-t border-line-soft pt-5">
      <h3 id={`${base}-heading`} className="m-0 font-heading text-base font-semibold">
        {settingsLabels.changePassword}
      </h3>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4">
        {fields.map((f) => {
          const id = `${base}-${f.key}`
          const error = errors[f.key]
          const describedBy = [error && `${id}-error`, f.hint && `${id}-hint`].filter(Boolean).join(' ') || undefined
          return (
            <Field key={f.key} id={id} label={f.label}>
              <TextInput
                id={id}
                type="password"
                autoComplete={f.autoComplete}
                value={value[f.key]}
                aria-invalid={!!error}
                aria-describedby={describedBy}
                onChange={(e) => {
                  setValue({ ...value, [f.key]: e.target.value })
                  setDone(false)
                }}
              />
              {f.hint && !error && (
                <span id={`${id}-hint`} className="text-xs text-text-muted">
                  {f.hint}
                </span>
              )}
              {error && (
                <span id={`${id}-error`} className="text-xs text-critical">
                  {error}
                </span>
              )}
            </Field>
          )
        })}
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" disabled={change.isPending}>
          {settingsLabels.changePassword}
        </Button>
        <div role="status" className="text-[13px]">
          {done && (
            <span className="inline-flex items-center gap-2 text-good">
              <Icon name="check" size={16} />
              Password changed
            </span>
          )}
          {change.error && <span className="text-critical">{change.error.message}</span>}
        </div>
      </div>
    </form>
  )
}
