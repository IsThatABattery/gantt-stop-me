import { useId, useState, type FormEvent } from 'react'
import { Button, Field, TextInput } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { errorCode } from '../../data/errors'
import { passwordProblems, useChangePassword, type PasswordInput } from '../../data/views/settings'
import { settingsLabels } from '../../labels/labels'

const empty: PasswordInput = { current: '', next: '', confirm: '' }

const fields: { key: keyof PasswordInput; label: string; autoComplete: string; hint?: string }[] = [
  { key: 'current', label: 'Current password', autoComplete: 'current-password' },
  { key: 'next', label: 'New password', autoComplete: 'new-password', hint: '8 or more characters.' },
  { key: 'confirm', label: 'Confirm new password', autoComplete: 'new-password' },
]

/** Checks the fields here first, then the API checks the current password. Other sessions are signed out. */
export function PasswordForm() {
  const [value, setValue] = useState(empty)
  const [errors, setErrors] = useState<Partial<Record<keyof PasswordInput, string>>>({})
  const [done, setDone] = useState(false)
  const change = useChangePassword()
  const base = useId()
  // The API's refusals that belong to one field (docs/api/accounts.md).
  const code = errorCode(change.error)
  const apiField: keyof PasswordInput | null = code === 'wrong_password' ? 'current' : code === 'weak_password' ? 'next' : null
  const shown = apiField && change.error ? { ...errors, [apiField]: change.error.message } : errors

  const submit = (e: FormEvent) => {
    e.preventDefault()
    change.reset()
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
    <form onSubmit={submit} noValidate aria-labelledby={`${base}-heading`} className="flex flex-col gap-4">
      <h3 id={`${base}-heading`} className="m-0 text-base font-bold">
        {settingsLabels.changePassword}
      </h3>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,200px),1fr))] gap-4">
        {fields.map((f) => {
          const id = `${base}-${f.key}`
          const error = shown[f.key]
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
                <span id={`${id}-hint`} className="text-xs text-ink-2">
                  {f.hint}
                </span>
              )}
              {error && (
                <span id={`${id}-error`} className="text-xs font-semibold text-signal-ink">
                  {error}
                </span>
              )}
            </Field>
          )
        })}
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" busy={change.isPending}>
          {settingsLabels.changePassword}
        </Button>
        <div role="status" className="text-[13px] font-semibold">
          {done && (
            <span className="inline-flex items-center gap-1.5 text-ink">
              <Icon name="check" size={16} className="text-good" />
              Password changed
            </span>
          )}
          {change.error && !apiField && (
            <span className="inline-flex items-center gap-1.5 text-signal-ink">
              <Icon name="alert" size={16} />
              {change.error.message}
            </span>
          )}
        </div>
      </div>
    </form>
  )
}
