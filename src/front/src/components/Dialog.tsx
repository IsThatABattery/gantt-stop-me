import { useId, type ReactNode } from 'react'
import { Icon } from './Icon'
import { useModal } from './useModal'

interface DialogProps {
  label: string
  /** Without it the dialog has no Close, and Esc or a click outside does nothing */
  onClose?: () => void
  children: ReactNode
  /** 860px, for long content such as a plan */
  wide?: boolean
  /**
   * An ink title strip across the top, holding the title and the close button, as the board's stage tabs do. The
   * dialog is then named by it. Without one, the content brings its own heading and the close button sits top right.
   */
  title?: ReactNode
}

const closeButton = 'absolute flex size-11 flex-none cursor-pointer items-center justify-center rounded-control border-0 bg-transparent transition-colors duration-150'

/** Centered dialog on paper, 560px wide, with a 1.5px ink edge. */
export function Dialog({ label, onClose, children, wide, title }: DialogProps) {
  const ref = useModal<HTMLDivElement>(onClose ?? (() => {}))
  const titleId = useId()
  return (
    <div className="fixed inset-0 z-40 flex items-start justify-center overflow-y-auto p-4">
      <div className="fixed inset-0 bg-ground/80" onClick={onClose} aria-hidden="true" />
      <div
        ref={ref}
        // my-auto, not items-center: a dialog taller than the screen scrolls from its top instead of being cut off.
        role="dialog"
        aria-modal="true"
        aria-label={title ? undefined : label}
        aria-labelledby={title ? titleId : undefined}
        tabIndex={-1}
        className={`relative w-full sm:my-auto ${wide ? 'max-w-[860px]' : 'max-w-[560px]'} rounded-dialog border-[1.5px] border-ink bg-paper text-ink shadow-xl outline-none`}
      >
        {title && (
          // pr-11 keeps the title clear of the close button, which sits over the strip's right end.
          <div className={`m-1.5 mb-0 flex min-h-11 items-center rounded-[3px] bg-ink pl-4 text-on-ink ${onClose ? 'pr-11' : 'pr-4'}`}>
            <h2 id={titleId} className="m-0 truncate font-display text-[22px] leading-none font-extrabold tracking-[0.04em] uppercase">
              {title}
            </h2>
          </div>
        )}
        {children}
        {/* After the children, so focus opens on the first field rather than Close. */}
        {onClose && (
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className={
              title
                ? `${closeButton} top-1.5 right-1.5 text-on-ink hover:bg-on-ink/15 focus-visible:outline-on-ink focus-visible:-outline-offset-[6px]`
                : `${closeButton} top-3 right-3 text-ink-2 hover:bg-ink/8 hover:text-ink`
            }
          >
            <Icon name="close" />
          </button>
        )}
      </div>
    </div>
  )
}
