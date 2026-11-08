import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router'
import { Button, Field, TextInput } from '../components/controls'
import { errorCode } from '../data/errors'
import { keys } from '../data/keys'
import { signUp, useMe } from '../data/session'
import { AuthFrame } from './LoginPage'

/** Which field each signup error belongs to (docs/accounts.md). */
const fieldOf: Record<string, 'email' | 'password'> = {
  invalid_email: 'email',
  email_taken: 'email',
  weak_password: 'password',
}

// Create an account with email and password (POST /api/auth/signup). No mockup;
// it follows the sign-in card.
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
      footer={
        <>
          Already have an account? <Link to="/login">Sign in</Link>
        </>
      }
    >
      <div>
        <h1 className="m-0 font-heading text-[26px] font-semibold">Create an account</h1>
      </div>

      <form onSubmit={onSubmit} noValidate className="flex flex-col gap-5">
        <div className="flex flex-col gap-3.5">
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
            {fieldError('email') && (
              <span id="signup-email-error" role="alert" className="text-xs text-critical">
                {fieldError('email')}
              </span>
            )}
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
              <span id="signup-password-error" role="alert" className="text-xs text-critical">
                {fieldError('password')}
              </span>
            ) : (
              <span id="signup-password-hint" className="text-xs text-text-muted">
                8 or more characters.
              </span>
            )}
          </Field>
        </div>

        {mutation.error && !field && (
          <p role="alert" className="m-0 text-[13px] text-critical">
            {mutation.error.message}
          </p>
        )}

        <Button type="submit" variant="primary" className="min-h-12" disabled={mutation.isPending}>
          {mutation.isPending ? 'Creating account…' : 'Create account'}
        </Button>
      </form>
    </AuthFrame>
  )
}
