import { useId, useState, type FormEvent } from 'react'
import { buttonClass } from '../../components/buttonClass'
import { Button, Field, TextInput } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { errorCode } from '../../data/errors'
import { GITHUB_REAUTH_URL, passwordProblems, useChangePassword, type PasswordInput } from '../../data/views/settings'
import { settingsLabels } from '../../labels/labels'

const empty: PasswordInput = { current: '', next: '', confirm: '' }

const fields: { key: keyof PasswordInput; label: string; autoComplete: string; hint?: string }[] = [
  { key: 'current', label: 'Current password', autoComplete: 'current-password' },
  { key: 'next', label: 'New password', autoComplete: 'new-password', hint: '8 or more characters.' },
  { key: 'confirm', label: 'Confirm new password', autoComplete: 'new-password' },
]

/**
 * Checks the fields here first, then the API checks the current password. Other sessions are signed out. An account
 * made by signing in with GitHub has no password (`hasPassword` false): the form sets one instead, with no current
 * password, which needs a GitHub sign-in in the last 10 minutes, so the refusal links to one.
 */
export function PasswordForm({ hasPassword }: { hasPassword: boolean }) {
  const [value, setValue] = useState(empty)
  const [errors, setErrors] = useState<Partial<Record<keyof PasswordInput, string>>>({})
  // What was done, kept as words: setting one refetches the user, which turns this into the change form.
  const [done, setDone] = useState<string | null>(null)
  const setting = !hasPassword
  const change = useChangePassword(setting)
  const base = useId()
  const title = setting ? settingsLabels.setPassword : settingsLabels.changePassword
  const shownFields = setting ? fields.filter((f) => f.key !== 'current') : fields
  // The API's refusals that belong to one field (docs/api/accounts.md).
  const code = errorCode(change.error)
  const fieldCode: keyof PasswordInput | null = code === 'wrong_password' ? 'current' : code === 'weak_password' ? 'next' : null
  // Only a field on show takes it; else it goes in the status line, so a refusal never goes unseen.
  const apiField = fieldCode && shownFields.some((f) => f.key === fieldCode) ? fieldCode : null
  const reauth = code === 'reauth_required'
  const shown = apiField && change.error ? { ...errors, [apiField]: change.error.message } : errors

  const submit = (e: FormEvent) => {
    e.preventDefault()
    change.reset()
    const problems = passwordProblems(value, setting)
    setErrors(problems)
    setDone(null)
    const first = shownFields.find((f) => problems[f.key])
    if (first) {
      document.getElementById(`${base}-${first.key}`)?.focus()
      return
    }
    change.mutate(value, {
      onSuccess: () => {
        setValue(empty)
        setDone(setting ? 'Password set' : 'Password changed')
      },
    })
  }

  return (
    <form onSubmit={submit} noValidate aria-labelledby={`${base}-heading`} className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <h3 id={`${base}-heading`} className="m-0 text-base font-bold">
          {title}
        </h3>
        {setting && (
          <p className="m-0 max-w-[60ch] text-[13px] leading-normal text-ink-2">
            You sign in with GitHub. A password lets you sign in with your email too.
          </p>
        )}
      </div>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,200px),1fr))] gap-4">
        {shownFields.map((f) => {
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
                  setDone(null)
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
          {title}
        </Button>
        <div role="status" className="flex min-w-0 flex-wrap items-center gap-x-3 text-[13px] font-semibold">
          {done && (
            <span className="inline-flex items-center gap-1.5 text-ink">
              <Icon name="check" size={16} className="text-good" />
              {done}
            </span>
          )}
          {change.error && !apiField && (
            <span className="inline-flex items-center gap-1.5 text-signal-ink">
              <Icon name="alert" size={16} className="flex-none" />
              {change.error.message}
            </span>
          )}
          {reauth && (
            // A full-page visit: the API signs in again through GitHub, then sends the user back to Settings.
            <a href={GITHUB_REAUTH_URL} className={buttonClass('link', 'px-0')}>
              <Icon name="github" size={16} className="flex-none" />
              Continue with GitHub
            </a>
          )}
        </div>
      </div>
    </form>
  )
}
