import { Navigate, Outlet, useLocation } from 'react-router'
import { useSession } from '../data/session'

/** Sends signed-out users to /login, then back to where they were going. */
export function RequireSession() {
  const session = useSession()
  const location = useLocation()
  if (!session) return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
  return <Outlet />
}
