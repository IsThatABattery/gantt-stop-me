import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, Navigate, useNavigate } from 'react-router'
import { Icon } from '../components/Icon'
import { Field, TextInput } from '../components/controls'
import { errorCode } from '../data/errors'
import { keys } from '../data/keys'
import { signUp, useMe } from '../data/session'
import { AuthError, AuthFrame, AuthSubmit, GitHubSignIn, OrWithEmail } from './LoginPage'

/** Which field each signup error belongs to (docs/api/accounts.md). */
const fieldOf: Record<string, 'email' | 'password'> = {
  invalid_email: 'email',
  email_taken: 'email',
  weak_password: 'password',
}

/** A field's refusal under it: the alert mark and the words, announced as it appears. */
function FieldError({ id, children }: { id: string; children: ReactNode }) {
  return (
    <span id={id} role="alert" className="flex items-start gap-1.5 text-[13px] leading-snug font-medium text-signal-ink">
      <Icon name="alert" size={14} className="mt-px flex-none" />
      {children}
    </span>
  )
}

// Create an account with email and password (POST /api/auth/signup), or with
// Continue with GitHub, in the same frame as sign in.
export function SignupPage() {
  const { data: me } = useMe()
  const client = useQueryClient()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const mutation = useMutation({
    mutationFn: signUp,
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.me })
      navigate('/', { replace: true })
    },
  })

  if (me && !mutation.isPending) return <Navigate to="/" replace />

  const code = errorCode(mutation.error)
  const field = code ? fieldOf[code] : undefined
  const fieldError = (name: 'email' | 'password') => (field === name ? mutation.error?.message : undefined)
  // Editing clears the refusal: it was about the old value.
  const clearError = () => mutation.isError && mutation.reset()

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    mutation.mutate({ email: email.trim(), password })
  }

  return (
    <AuthFrame
      title="Create an account"
      footer={
        <>
          Already have an account? <Link to="/login">Sign in</Link>
        </>
      }
    >
      <GitHubSignIn className="gap-2">
        <p className="m-0 text-[13px] leading-normal text-ink-2">Signs you up with your GitHub email, and links GitHub at once.</p>
        {/* gap-2 and mt-3 make the panel's gap-5. */}
        <OrWithEmail className="mt-3" />
      </GitHubSignIn>

      <p className="m-0 text-sm leading-normal text-ink-2">
        Sign up with your email. You'll link GitHub next, so the board can read the repository's issues.
      </p>

      <form onSubmit={onSubmit} noValidate className="flex flex-col gap-5">
        <div className="flex flex-col gap-4">
          <Field id="signup-email" label="Email">
            <TextInput
              id="signup-email"
              type="email"
              autoComplete="email"
              placeholder="you@team.dev"
              value={email}
              aria-invalid={!!fieldError('email')}
              aria-describedby={fieldError('email') ? 'signup-email-error' : undefined}
              onChange={(e) => {
                setEmail(e.target.value)
                clearError()
              }}
            />
            {fieldError('email') && <FieldError id="signup-email-error">{fieldError('email')}</FieldError>}
          </Field>
          <Field id="signup-password" label="Password">
            <TextInput
              id="signup-password"
              type="password"
              autoComplete="new-password"
              value={password}
              aria-invalid={!!fieldError('password')}
              aria-describedby={fieldError('password') ? 'signup-password-error' : 'signup-password-hint'}
              onChange={(e) => {
                setPassword(e.target.value)
                clearError()
              }}
            />
            {fieldError('password') ? (
              <FieldError id="signup-password-error">{fieldError('password')}</FieldError>
            ) : (
              <span id="signup-password-hint" className="text-[13px] text-ink-2">
                8 or more characters.
              </span>
            )}
          </Field>
        </div>

        {mutation.error && !field && <AuthError>{mutation.error.message}</AuthError>}

        <AuthSubmit busy={mutation.isPending}>{mutation.isPending ? 'Creating account…' : 'Create account'}</AuthSubmit>
      </form>
    </AuthFrame>
  )
}
