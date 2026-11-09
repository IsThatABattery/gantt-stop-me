import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router'
import { Icon } from '../components/Icon'
import { Logo } from '../components/Logo'
import { Button, Checkbox, Field, TextInput } from '../components/controls'
import { keys } from '../data/keys'
import { signIn, useMe } from '../data/session'
import { labels } from '../labels/labels'

// Matches Login.dc.html. Email and password sign in against the API.
// GitHub, SSO, Forgot password and Keep me signed in are UI only.
export function LoginPage() {
  const { data: me } = useMe()
  const client = useQueryClient()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from ?? '/'
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const mutation = useMutation({
    mutationFn: signIn,
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: keys.me })
      navigate(from, { replace: true })
    },
  })

  if (me && !mutation.isPending) return <Navigate to={from} replace />

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    mutation.mutate({ email: email.trim(), password })
  }

  return (
    <AuthFrame
      footer={
        <>
          {labels.newUser} <Link to="/signup">{labels.requestAccess}</Link>
        </>
      }
    >
      <div>
        <h1 className="m-0 font-heading text-[26px] font-semibold">Sign in</h1>
      </div>

      <div className="flex flex-col gap-2.5">
        <Button className="min-h-[46px] bg-surface-sunken font-medium">
          <Icon name="github" />
          Continue with GitHub
        </Button>
        <Button className="min-h-[46px] bg-surface-sunken font-medium">
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
        <div className="flex flex-col gap-3.5">
          <Field id="email" label="Email">
            <TextInput
              id="email"
              type="email"
              autoComplete="email"
              placeholder="you@team.dev"
              required
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
            <TextInput
              id="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          <Checkbox label="Keep me signed in" />
        </div>

        {mutation.error && (
          <p role="alert" className="m-0 text-[13px] text-critical">
            {mutation.error.message}
          </p>
        )}

        <Button type="submit" variant="primary" className="min-h-12" disabled={mutation.isPending}>
          {mutation.isPending ? 'Signing in…' : 'Sign in'}
        </Button>
      </form>
    </AuthFrame>
  )
}

/** The starfield page with the logo, one card and a line under it. Shared by sign in and sign up. */
export function AuthFrame({ children, footer }: { children: ReactNode; footer: ReactNode }) {
  return (
    // Anchored to the top, not centred, so an error appearing in the card doesn't move it.
    <div className="stars box-border flex min-h-screen flex-col items-center gap-7 bg-background p-6 pt-[12vh] text-[15px] text-text">
      <Link to="/login" className="flex min-h-11 items-center no-underline">
        <Logo size={32} textSize="text-[22px]" />
      </Link>
      <div className="box-border flex w-full max-w-[420px] flex-col gap-5 rounded-dialog border border-line bg-surface/95 p-8">{children}</div>
      <div className="text-sm text-text-muted">{footer}</div>
    </div>
  )
}
