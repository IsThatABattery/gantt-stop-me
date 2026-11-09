import { useEffect, useId, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { useMe, useSignOut } from '../data/session'
import { UserAvatar } from './Avatar'
import { Icon } from './Icon'

/**
 * The signed-in person's avatar and email as a full-width row at the foot of the rail, with a menu above it holding
 * Sign out. Opening it focuses Sign out; Esc closes it and returns focus to the button.
 */
export function UserMenu() {
  const { data: me } = useMe()
  const signOut = useSignOut()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const button = useRef<HTMLButtonElement>(null)
  const menuId = useId()
  const navigate = useNavigate()

  useEffect(() => {
    if (!open) return
    ref.current?.querySelector<HTMLElement>('[role=menuitem]')?.focus()
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      setOpen(false)
      button.current?.focus()
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  if (!me) return null
  // The API has no names yet, so the avatar shows the email's first letters.
  const initials = me.email.slice(0, 2).toUpperCase()

  return (
    <div ref={ref} className="relative w-full">
      <button
        ref={button}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        aria-label={`Account menu, signed in as ${me.email}`}
        onClick={() => setOpen((o) => !o)}
        className="flex min-h-11 w-full cursor-pointer items-center gap-2.5 rounded-[3px] border-0 bg-transparent px-2.5 text-left font-[inherit] text-ink transition-colors duration-150 hover:bg-block aria-expanded:bg-block"
      >
        <UserAvatar initials={initials} />
        <span className="min-w-0 flex-1 truncate text-[13px] text-ink-2" title={me.email}>
          {me.email}
        </span>
        <Icon name="chevronDown" size={14} className="flex-none rotate-180 text-ink-2" />
      </button>
      {open && (
        <div
          id={menuId}
          role="menu"
          // It spans the rail's width, so the rail's scroll box never clips it and it never runs past the edge.
          className="absolute inset-x-0 bottom-full z-20 mb-2 flex flex-col gap-1 rounded-card border-[1.5px] border-ink bg-paper p-1.5 shadow-lg"
        >
          <div className="px-2.5 py-2 text-xs text-ink-2">
            Signed in as
            <div className="truncate text-sm font-semibold text-ink" title={me.email}>
              {me.email}
            </div>
          </div>
          <button
            type="button"
            role="menuitem"
            onClick={async () => {
              await signOut()
              // In sync: in a transition, RequireSession would first render the cleared session as one that ended.
              navigate('/login', { replace: true, flushSync: true })
            }}
            className="flex min-h-11 cursor-pointer items-center gap-2.5 rounded-[3px] border-0 bg-transparent px-2.5 text-left font-[inherit] font-semibold text-ink hover:bg-ink/8 active:bg-ink/15"
          >
            <Icon name="signOut" />
            Sign out
          </button>
        </div>
      )}
    </div>
  )
}
