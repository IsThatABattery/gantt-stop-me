import { Navigate, Outlet, useLocation } from 'react-router'
import { StatusMessage } from '../components/StatusMessage'
import { Button } from '../components/controls'
import { useMe } from '../data/session'

/** Sends signed-out users to /login, then back to where they were going. */
export function RequireSession() {
  const { data: me, isPending, error, refetch } = useMe()
  const location = useLocation()

  if (isPending || error) {
    return (
      <div className="stars flex min-h-screen flex-col bg-background text-text">
        {isPending ? (
          <StatusMessage title="Loading…" />
        ) : (
          <StatusMessage title="Something went wrong" body={error.message}>
            <Button onClick={() => refetch()}>Try again</Button>
          </StatusMessage>
        )}
      </div>
    )
  }
  if (!me) return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
  return <Outlet />
}
