import { cx } from '../lib/cx'

export type ButtonVariant = 'primary' | 'secondary' | 'accent' | 'link'

const variants: Record<ButtonVariant, string> = {
  primary: 'bg-ion text-on-ion font-semibold border border-ion hover:bg-ion-hover hover:text-on-ion',
  secondary: 'border border-line bg-transparent text-text hover:border-line-strong hover:text-text',
  accent: 'border border-ion bg-ion/8 text-text hover:bg-ion/15 hover:text-text',
  link: 'border border-transparent text-ion hover:text-ion-hover',
}

/** Shared button classes, so links can look like buttons. */
export function buttonClass(variant: ButtonVariant = 'secondary', extra = '') {
  return cx(
    `inline-flex min-h-11 cursor-pointer items-center justify-center gap-2 rounded-control px-4 font-[inherit] no-underline ${variants[variant]}`,
    extra,
  )
}
