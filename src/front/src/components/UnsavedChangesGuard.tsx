import { useEffect, useId } from 'react'
import { useBlocker } from 'react-router'
import { Button } from './controls'
import { Icon } from './Icon'

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
  // focus() also scrolls Stay into view, so the click on a link visibly lands here.
  useEffect(() => {
    if (blocker.state === 'blocked') document.getElementById(`${id}-stay`)?.focus()
  }, [blocker.state, id])

  if (blocker.state !== 'blocked') return null
  return (
    <div role="alertdialog" aria-labelledby={id} className="flex flex-wrap items-center gap-3 rounded-card border-[1.5px] border-ink bg-paper p-3">
      <span aria-hidden="true" className="grid size-9 flex-none place-items-center rounded-[2px] bg-approve text-on-approve">
        <Icon name="alert" size={18} />
      </span>
      <p id={id} className="m-0 min-w-[12rem] flex-1 text-sm font-semibold">
        You have unsaved changes. Leave without saving?
      </p>
      <Button id={`${id}-stay`} onClick={() => blocker.reset()}>
        Stay
      </Button>
      <Button variant="danger" onClick={() => blocker.proceed()}>
        Leave without saving
      </Button>
    </div>
  )
}
