import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router'
import { Dialog } from '../components/Dialog'
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
      <div className="stars flex min-h-screen flex-col bg-background text-text">
        {isPending ? (
          <StatusMessage title="Loading…" />
        ) : (
          <StatusMessage title="Something went wrong" body={error?.message}>
            <Button onClick={() => refetch()}>Try again</Button>
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
    <Dialog label="Session ended" onClose={leave}>
      <form onSubmit={onSubmit} className="flex flex-col gap-5 p-7">
        <div className="pr-10">
          <h2 className="m-0 font-heading text-2xl font-semibold">Your session ended</h2>
          <p className="mt-2 mb-0 leading-normal wrap-anywhere text-text-muted-strong">
            Sign in again as <span className="text-text">{email}</span> to carry on. This page keeps what you typed.
          </p>
        </div>
        <Field id="session-password" label="Password">
          <TextInput
            id="session-password"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </Field>
        {mutation.error && (
          <p role="alert" className="m-0 text-[13px] wrap-anywhere text-critical">
            {mutation.error.message}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2.5">
          <Button onClick={leave}>Use another account</Button>
          <Button type="submit" variant="primary" disabled={mutation.isPending}>
            {mutation.isPending ? 'Signing in…' : 'Sign in'}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
