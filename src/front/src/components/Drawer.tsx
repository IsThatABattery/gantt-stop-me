import type { ReactNode } from 'react'
import { Icon } from './Icon'
import { useModal } from './useModal'

interface DrawerProps {
  /** Read by screen readers as the drawer's name, and shown in its ink tab unless title says otherwise */
  label: string
  /** What the ink tab shows, when it differs from label (e.g. a task's issue number) */
  title?: ReactNode
  onClose: () => void
  /** Pinned below the scrolling body, e.g. the approval bar */
  footer?: ReactNode
  children: ReactNode
}

/** A paper sheet from the right, 560px wide, topped by an ink tab holding its name and the close button. */
export function Drawer({ label, title, onClose, footer, children }: DrawerProps) {
  const ref = useModal<HTMLDivElement>(onClose)
  return (
    <div className="fixed inset-0 z-40 flex justify-end">
      <div className="absolute inset-0 bg-ground/80" onClick={onClose} aria-hidden="true" />
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        tabIndex={-1}
        className="relative flex h-full w-full max-w-[560px] flex-col border-l border-ink bg-paper text-ink shadow-xl outline-none"
      >
        <div className="m-1.5 mb-0 flex min-h-11 flex-none items-center justify-between gap-3 rounded-[3px] bg-ink pl-4 text-on-ink">
          <span className="truncate font-display text-[22px] leading-none font-extrabold tracking-[0.04em] uppercase" aria-hidden="true">
            {title ?? label}
          </span>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="flex size-11 flex-none cursor-pointer items-center justify-center rounded-control border-0 bg-transparent text-on-ink transition-colors duration-150 hover:bg-on-ink/15 focus-visible:outline-on-ink focus-visible:-outline-offset-[6px]"
          >
            <Icon name="close" />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
        {footer && <div className="border-t border-rule bg-ground">{footer}</div>}
      </div>
    </div>
  )
}
