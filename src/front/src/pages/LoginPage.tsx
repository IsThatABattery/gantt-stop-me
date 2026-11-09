import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useId, useState, type CSSProperties, type FormEvent, type ReactNode } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router'
import { Icon } from '../components/Icon'
import { LogoMark } from '../components/Logo'
import { PageTitle } from '../components/PageTitle'
import { InkTab } from '../components/Tile'
import { tileStep } from '../components/tileStep'
import { Button, ButtonMark, Checkbox, Field, TextInput } from '../components/controls'
import { keys } from '../data/keys'
import { signIn, useMe } from '../data/session'
import { COLUMN_ORDER } from '../data/types'
import { columnLabels, columnPurposeLabels, labels } from '../labels/labels'

// Email and password sign in against the API. GitHub, SSO, Forgot password and
// Keep me signed in are UI only (docs/implementation/api-gaps.md).
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
      title="Sign in"
      footer={
        <>
          {labels.newUser} <Link to="/signup">{labels.requestAccess}</Link>
        </>
      }
    >
      <div className="flex flex-col gap-2">
        <Button className="w-full">
          <Icon name="github" />
          Continue with GitHub
        </Button>
        <Button className="w-full">
          <Icon name="lock" />
          Continue with SSO
        </Button>
      </div>

      <div className="flex items-center gap-3 text-[13px] text-ink-2">
        <span className="h-px flex-1 bg-rule" />
        or with email
        <span className="h-px flex-1 bg-rule" />
      </div>

      <form onSubmit={onSubmit} className="flex flex-col gap-5">
        <div className="flex flex-col gap-4">
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
              <a href="#" onClick={(e) => e.preventDefault()} className="text-[13px] font-semibold">
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

        {mutation.error && <AuthError>{mutation.error.message}</AuthError>}

        <AuthSubmit busy={mutation.isPending}>{mutation.isPending ? 'Signing in…' : 'Sign in'}</AuthSubmit>
      </form>
    </AuthFrame>
  )
}

const stagePurpose = columnPurposeLabels

/**
 * The board's stages as small stepped tiles under ink tabs, read left to right; the stage where a person approves
 * carries the approve fill. Built from the column order, so it follows however many stages there are.
 */
function StageStrip() {
  return (
    <ol aria-label="How a task moves" className="m-0 grid list-none auto-cols-fr grid-flow-col gap-1.5 p-0">
      {COLUMN_ORDER.map((key, i) => {
        const person = key === 'user-check'
        return (
          <li
            key={key}
            className="tile flex min-w-0 flex-col gap-1.5 rounded-[3px] p-1"
            style={{ '--tile-step': tileStep(i, COLUMN_ORDER.length) } as CSSProperties}
          >
            <span className="flex h-7 items-center rounded-[2px] bg-ink px-2 text-on-ink">
              <span className="truncate font-display text-[15px] leading-none font-extrabold tracking-[0.045em] uppercase">{columnLabels[key].name}</span>
            </span>
            <span
              className={`flex min-h-10 items-start gap-1 rounded-[2px] px-1.5 py-1 text-xs leading-tight ${
                person ? 'bg-approve font-semibold text-on-approve' : 'text-ink-2'
              }`}
            >
              {person && <Icon name="person" size={12} className="mt-px flex-none" />}
              {stagePurpose[key]}
            </span>
          </li>
        )
      })}
    </ol>
  )
}

/** A refusal from the API: an alert mark and the words, on the signal wash. */
export function AuthError({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="m-0 flex items-start gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] leading-snug font-medium text-signal-ink">
      <Icon name="alert" size={16} className="mt-px flex-none" />
      {children}
    </p>
  )
}

/** The panel's one ink action, full width, led by the paper mark as on the board's New task button. */
export function AuthSubmit({ busy, children }: { busy: boolean; children: ReactNode }) {
  return (
    <Button type="submit" variant="primary" busy={busy} className="min-h-[52px] w-full pl-2 pr-[18px] text-[15px]">
      <ButtonMark icon="chevronRight" />
      <span className="flex-1">{children}</span>
      <span aria-hidden="true" className="w-9 flex-none" />
    </Button>
  )
}

/**
 * The ground with the product name in display caps and how a task moves, beside a paper panel under an ink tab that
 * holds the form. Shared by sign in and sign up.
 */
export function AuthFrame({ title, children, footer }: { title: string; children: ReactNode; footer: ReactNode }) {
  const titleId = useId()
  return (
    <div className="stars box-border flex min-h-screen flex-col bg-ground text-ink">
      <main className="mx-auto box-border grid w-full max-w-[1180px] flex-1 content-center items-center gap-x-20 gap-y-8 px-4 py-8 sm:px-8 sm:py-12 lg:grid-cols-[minmax(0,1fr)_440px]">
        <div className="flex min-w-0 flex-col gap-4 lg:gap-6">
          <Link to="/login" aria-label={labels.appName} className="flex size-11 items-center rounded-[3px] no-underline lg:size-14">
            <LogoMark size={44} />
          </Link>
          <PageTitle size="lg" className="lg:text-[88px]">
            {labels.appName}
          </PageTitle>
          <p className="m-0 max-w-[42ch] text-[15px] leading-normal text-ink-2 lg:text-[17px]">
            A board over your GitHub repository. Tests run on each task's pull request, and nothing merges until a person approves it.
          </p>
          <div className="mt-2 max-md:hidden lg:mt-4">
            <StageStrip />
          </div>
        </div>

        <div className="flex min-w-0 flex-col gap-4">
          <section aria-labelledby={titleId} className="rounded-dialog border-[1.5px] border-ink bg-paper p-1.5">
            <InkTab id={titleId}>{title}</InkTab>
            <div className="flex flex-col gap-5 px-4 pt-6 pb-5 sm:px-6 sm:pb-6">{children}</div>
          </section>
          <p className="m-0 text-center text-sm text-ink-2">{footer}</p>
        </div>
      </main>
    </div>
  )
}
