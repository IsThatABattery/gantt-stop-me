import { useEffect, useId, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { useMe, useSignOut } from '../data/session'
import { UserAvatar } from './Avatar'
import { Icon } from './Icon'

interface UserMenuProps {
  /** The menu opens below the button, or above it at the foot of the rail */
  placement?: 'below' | 'above'
  /** Show the signed-in email beside the avatar, as a full-width row (the rail's foot) */
  showEmail?: boolean
}

/**
 * The signed-in person's avatar, with a menu holding Sign out. Opening it focuses Sign out; Esc closes it and returns
 * focus to the avatar.
 */
export function UserMenu({ placement = 'below', showEmail = false }: UserMenuProps) {
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
    <div ref={ref} className={`relative ${showEmail ? 'w-full' : ''}`}>
      <button
        ref={button}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        aria-label={`Account menu, signed in as ${me.email}`}
        onClick={() => setOpen((o) => !o)}
        className={`flex min-h-11 cursor-pointer items-center gap-2.5 rounded-[3px] border-0 bg-transparent font-[inherit] text-ink transition-colors duration-150 hover:bg-block aria-expanded:bg-block ${
          showEmail ? 'w-full px-2.5 text-left' : 'min-w-11 justify-center p-0'
        }`}
      >
        <UserAvatar user={{ initials, role: 'member' }} size={showEmail ? 'md' : 'lg'} />
        {showEmail && (
          <>
            <span className="min-w-0 flex-1 truncate text-[13px] text-ink-2" title={me.email}>
              {me.email}
            </span>
            <Icon name="chevronDown" size={14} className={`flex-none text-ink-2 transition-transform duration-150 ${placement === 'above' ? 'rotate-180' : ''}`} />
          </>
        )}
      </button>
      {open && (
        <div
          id={menuId}
          role="menu"
          // In the rail it spans the rail's width, so the rail's scroll box never clips it and it never runs past the edge.
          className={`absolute z-20 flex ${placement === 'above' ? 'bottom-full mb-2' : 'top-full mt-2'} ${
            showEmail ? 'inset-x-0' : `w-60 ${placement === 'above' ? 'left-0' : 'right-0'}`
          } flex-col gap-1 rounded-card border-[1.5px] border-ink bg-paper p-1.5 shadow-lg`}
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
