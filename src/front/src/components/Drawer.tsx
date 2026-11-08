import type { ReactNode } from 'react'
import { Icon } from './Icon'
import { useModal } from './useModal'

interface DrawerProps {
  /** Read by screen readers as the drawer's name */
  label: string
  onClose: () => void
  /** Pinned below the scrolling body, e.g. the approval bar */
  footer?: ReactNode
  children: ReactNode
}

/** Right-side panel over the page, 560px wide (SCREENS.md layout notes). */
export function Drawer({ label, onClose, footer, children }: DrawerProps) {
  const ref = useModal<HTMLDivElement>(onClose)
  return (
    <div className="fixed inset-0 z-40 flex justify-end">
      <div className="absolute inset-0 bg-background/70" onClick={onClose} aria-hidden="true" />
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        tabIndex={-1}
        className="relative flex h-full w-full max-w-[560px] flex-col border-l border-line bg-surface text-text shadow-2xl outline-none"
      >
        <button
          type="button"
          onClick={onClose}
          aria-label="Close"
          className="absolute top-3 right-3 z-10 flex size-11 cursor-pointer items-center justify-center rounded-control border-0 bg-transparent text-text-muted-strong hover:text-text"
        >
          <Icon name="close" />
        </button>
        <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
        {footer && <div className="border-t border-line bg-surface-sunken">{footer}</div>}
      </div>
    </div>
  )
}
