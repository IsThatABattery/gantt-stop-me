import { useMutation } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router'
import { Eyebrow } from '../components/Eyebrow'
import { Icon } from '../components/Icon'
import { Logo } from '../components/Logo'
import { Button, Checkbox, Field, TextInput } from '../components/controls'
import { signIn } from '../data/core'
import { saveSession, useSession } from '../data/session'
import { labels } from '../labels/labels'

// Matches Login.dc.html. Only "Sign in" works for now (mock auth, no checks).
// GitHub, SSO, Forgot password, Keep me signed in and Request access are UI only.
export function LoginPage() {
  const session = useSession()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from ?? '/'
  const [email, setEmail] = useState('')

  const mutation = useMutation({
    mutationFn: signIn,
    onSuccess: (s) => {
      saveSession(s)
      navigate(from, { replace: true })
    },
  })

  if (session && !mutation.isPending) return <Navigate to={from} replace />

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    mutation.mutate(email)
  }

  return (
    <div className="stars box-border flex min-h-screen flex-col items-center justify-center gap-7 bg-background p-6 text-[15px] text-text">
      <Link to="/login" className="flex min-h-11 items-center no-underline">
        <Logo size={32} textSize="text-[22px]" />
      </Link>

      <div className="box-border flex w-full max-w-[420px] flex-col gap-5 rounded-dialog border border-line bg-surface/95 p-8">
        <div>
          <Eyebrow>{labels.flightDeckAccess}</Eyebrow>
          <h1 className="mt-2 mb-0 font-heading text-[26px] font-semibold">Sign in</h1>
        </div>

        <div className="flex flex-col gap-3">
          <Button className="min-h-12 bg-surface-sunken font-medium">
            <Icon name="github" />
            Continue with GitHub
          </Button>
          <Button className="min-h-12 bg-surface-sunken font-medium">
            <Icon name="lock" />
            Continue with SSO
          </Button>
        </div>

        <div className="flex items-center gap-3 text-[13px] text-text-muted">
          <span className="h-px flex-1 bg-line" />
          or with email
          <span className="h-px flex-1 bg-line" />
        </div>

        <form onSubmit={onSubmit} className="flex flex-col gap-5">
          <div className="flex flex-col gap-4">
            <Field id="email" label="Email">
              <TextInput
                id="email"
                type="email"
                autoComplete="email"
                placeholder="you@team.dev"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </Field>
            <Field
              id="password"
              label="Password"
              aside={
                <a href="#" onClick={(e) => e.preventDefault()} className="text-[13px]">
                  Forgot password?
                </a>
              }
            >
              <TextInput id="password" type="password" autoComplete="current-password" />
            </Field>
            <Checkbox label="Keep me signed in" />
          </div>

          <Button type="submit" variant="primary" className="min-h-12" disabled={mutation.isPending}>
            {mutation.isPending ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
      </div>

      <div className="text-sm text-text-muted">
        {labels.newUser}{' '}
        <a href="#" onClick={(e) => e.preventDefault()}>
          {labels.requestAccess}
        </a>
      </div>
    </div>
  )
}
