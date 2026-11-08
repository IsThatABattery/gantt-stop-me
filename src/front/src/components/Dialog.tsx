import type { ReactNode } from 'react'
import { Icon } from './Icon'
import { useModal } from './useModal'

interface DialogProps {
  label: string
  onClose: () => void
  children: ReactNode
}

/** Centered dialog, 560px wide (SCREENS.md layout notes). */
export function Dialog({ label, onClose, children }: DialogProps) {
  const ref = useModal<HTMLDivElement>(onClose)
  return (
    <div className="fixed inset-0 z-40 flex items-start justify-center overflow-y-auto p-4 sm:items-center">
      <div className="fixed inset-0 bg-background/70" onClick={onClose} aria-hidden="true" />
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        tabIndex={-1}
        className="relative w-full max-w-[560px] rounded-dialog border border-line bg-surface text-text shadow-2xl outline-none"
      >
        <button
          type="button"
          onClick={onClose}
          aria-label="Close"
          className="absolute top-3 right-3 flex size-11 cursor-pointer items-center justify-center rounded-control border-0 bg-transparent text-text-muted-strong hover:text-text"
        >
          <Icon name="close" />
        </button>
        {children}
      </div>
    </div>
  )
}
