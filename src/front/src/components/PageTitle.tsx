import type { ReactNode } from 'react'
import { cx } from '../lib/cx'

const sizes = {
  /** The board: the project name, set huge */
  lg: 'text-[44px] md:text-[58px]',
  /** Every other page: Settings, Automation, Backlog */
  md: 'text-[36px] md:text-[44px]',
}

interface PageTitleProps {
  children: ReactNode
  size?: keyof typeof sizes
  id?: string
  className?: string
}

/** A page's h1 in Big Shoulders Display caps. Only page titles and stage tabs use the display face. */
export function PageTitle({ children, size = 'md', id, className = '' }: PageTitleProps) {
  return (
    <h1 id={id} className={cx(`m-0 font-display leading-[0.86] font-black tracking-[0.005em] break-words uppercase ${sizes[size]}`, className)}>
      {children}
    </h1>
  )
}
