import { useEffect, useId, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { useUser } from '../data/core'
import { signOut, useSession } from '../data/session'
import { UserAvatar } from './Avatar'
import { Icon } from './Icon'

/** Signed-in avatar with a menu holding Sign out. */
export function UserMenu() {
  const session = useSession()
  const { data: user } = useUser(session?.userId)
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const menuId = useId()
  const navigate = useNavigate()

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  if (!session || !user) return null

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        aria-label={`Account menu, signed in as ${session.email}`}
        onClick={() => setOpen((o) => !o)}
        className="flex size-11 cursor-pointer items-center justify-center rounded-full border-0 bg-transparent p-0"
      >
        <UserAvatar user={user} size="lg" />
      </button>
      {open && (
        <div
          id={menuId}
          role="menu"
          className="absolute top-full right-0 z-20 mt-2 flex w-60 flex-col gap-1 rounded-card border border-line bg-surface p-2 shadow-xl"
        >
          <div className="px-3 py-2 text-xs text-text-muted">
            Signed in as
            <div className="truncate text-sm text-text">{session.email}</div>
          </div>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              signOut()
              navigate('/login', { replace: true })
            }}
            className="flex min-h-11 cursor-pointer items-center gap-3 rounded-lg border-0 bg-transparent px-3 text-left font-[inherit] text-text hover:bg-nav-active"
          >
            <Icon name="signOut" />
            Sign out
          </button>
        </div>
      )}
    </div>
  )
}
