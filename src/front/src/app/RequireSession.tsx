import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router'
import { Dialog } from '../components/Dialog'
import { Icon } from '../components/Icon'
import { StatusMessage } from '../components/StatusMessage'
import { Button, Field, TextInput } from '../components/controls'
import { forgetSession, signIn, useMe } from '../data/session'

/**
 * Sends signed-out users to /login, then back to where they were going. When the session ends mid-visit (a 401),
 * the page stays under a dialog to sign in again, so nothing typed on it is lost.
 */
export function RequireSession() {
  const { data: me, isPending, error, refetch } = useMe()
  const location = useLocation()
  const [lastEmail, setLastEmail] = useState<string>()
  if (me && me.email !== lastEmail) setLastEmail(me.email)

  // A failed background refetch keeps the signed-in page.
  if (isPending || (error && me === undefined)) {
    return (
      <div className="stars flex min-h-screen flex-col bg-ground text-ink">
        {isPending ? (
          <StatusMessage title="Loading…" />
        ) : (
          <StatusMessage tone="signal" title="Something went wrong" body={error?.message}>
            <Button variant="primary" onClick={() => refetch()}>
              Try again
            </Button>
          </StatusMessage>
        )}
      </div>
    )
  }
  if (!me && !lastEmail) return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
  return (
    <>
      <Outlet />
      {!me && lastEmail && <SessionEnded email={lastEmail} />}
    </>
  )
}

function SessionEnded({ email }: { email: string }) {
  const client = useQueryClient()
  const navigate = useNavigate()
  const location = useLocation()
  const [password, setPassword] = useState('')
  // Refetch everything: what failed with the 401 comes back, and the page keeps its unsaved edits.
  const mutation = useMutation({ mutationFn: signIn, onSuccess: () => client.invalidateQueries() })
  const leave = () => {
    // Off the page first, so nothing on it refetches into the cache being forgotten.
    navigate('/login', { replace: true, flushSync: true, state: { from: location.pathname + location.search } })
    forgetSession(client)
  }
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    mutation.mutate({ email, password })
  }

  return (
    // No onClose: Esc or a click outside must not leave the page it promises to keep.
    <Dialog label="Session ended" title="Session ended">
      <form onSubmit={onSubmit} className="flex flex-col gap-5 px-6 pt-5 pb-6 max-sm:px-4">
        <p className="m-0 text-sm leading-normal wrap-anywhere text-ink-2">
          Sign in again as <span className="font-semibold text-ink">{email}</span> to carry on. This page keeps what you typed.
        </p>
        <Field id="session-password" label="Password">
          <TextInput
            id="session-password"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            aria-describedby={mutation.isError ? 'session-error' : undefined}
          />
        </Field>
        {mutation.error && (
          <p
            id="session-error"
            role="alert"
            className="m-0 flex items-start gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] leading-snug font-medium wrap-anywhere text-signal-ink"
          >
            <Icon name="alert" size={16} className="mt-px flex-none" />
            {mutation.error.message}
          </p>
        )}
        <div className="-mx-6 flex flex-wrap justify-end gap-2.5 border-t border-rule px-6 pt-5 max-sm:-mx-4 max-sm:px-4">
          <Button onClick={leave}>Use another account</Button>
          <Button type="submit" variant="primary" busy={mutation.isPending} className="px-[22px]">
            {mutation.isPending ? 'Signing in…' : 'Sign in'}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
