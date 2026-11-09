import { cx } from '../lib/cx'

/**
 * primary: the solid ink action, one per view (New task, Save, Approve).
 * secondary: paper with a 1.5px ink edge, for every other action.
 * ghost: no fill or edge until hovered, for quiet actions in toolbars and lists.
 * danger: a signal fill, for an action that throws work away (Leave without saving, Delete).
 * link: a text link at button height.
 */
export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'link'

const variants: Record<ButtonVariant, string> = {
  primary: 'border-ink bg-ink font-bold text-on-ink hover:border-ink-hover hover:bg-ink-hover hover:text-on-ink active:border-ink active:bg-ink',
  secondary: 'border-ink bg-paper font-semibold text-ink hover:bg-ink/8 hover:text-ink active:bg-ink/15',
  ghost: 'border-transparent bg-transparent font-semibold text-ink hover:bg-ink/8 hover:text-ink active:bg-ink/15',
  danger: 'border-signal bg-signal font-bold text-on-signal hover:border-signal-ink hover:bg-signal-ink hover:text-on-signal active:bg-signal',
  link: 'border-transparent bg-transparent font-semibold text-accent underline decoration-1 underline-offset-[3px] hover:text-ink hover:decoration-2',
}

/** Shared button classes, so links can look like buttons. 44px tall; pass min-h-[52px] for the board's header actions. */
export function buttonClass(variant: ButtonVariant = 'secondary', extra = '') {
  return cx(
    `inline-flex min-h-11 cursor-pointer items-center justify-center gap-2 rounded-control border-[1.5px] ${variant === 'link' ? 'px-1' : 'px-4 no-underline'} font-[inherit] text-sm leading-tight transition-colors duration-150 ease-out-quint disabled:cursor-not-allowed disabled:opacity-45 aria-busy:cursor-progress ${variants[variant]}`,
    extra,
  )
}
