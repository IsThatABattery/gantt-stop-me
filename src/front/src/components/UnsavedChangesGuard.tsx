import { useEffect, useId } from 'react'
import { useBlocker } from 'react-router'
import { Button } from './controls'

/**
 * Asks before leaving the page, or closing the tab, while `when` holds. Hash
 * changes pass, so in-page section links still work. A router allows one
 * blocker, so mount at most one per page.
 */
export function UnsavedChangesGuard({ when }: { when: boolean }) {
  const id = useId()
  const blocker = useBlocker(({ currentLocation, nextLocation }) => when && currentLocation.pathname !== nextLocation.pathname)
  useEffect(() => {
    if (!when) return
    const onUnload = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener('beforeunload', onUnload)
    return () => window.removeEventListener('beforeunload', onUnload)
  }, [when])

  if (blocker.state !== 'blocked') return null
  return (
    <div role="alertdialog" aria-labelledby={id} className="flex flex-wrap items-center gap-3 rounded-card border border-warning bg-warning/6 p-4">
      <p id={id} className="m-0 flex-1 text-[13px]">
        You have unsaved changes. Leave without saving?
      </p>
      <Button onClick={() => blocker.reset()}>Stay</Button>
      <Button onClick={() => blocker.proceed()} className="border-warning text-warning hover:border-warning hover:text-warning">
        Leave without saving
      </Button>
    </div>
  )
}
