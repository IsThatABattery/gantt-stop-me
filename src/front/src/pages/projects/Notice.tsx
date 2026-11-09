import type { ReactNode } from 'react'
import { Icon, type IconName } from '../../components/Icon'

interface NoticeProps {
  title: string
  children?: ReactNode
  /** Buttons or links under the words */
  actions?: ReactNode
  /** signal when something failed: a signal edge and a filled signal mark. Else a hollow ink mark. */
  tone?: 'neutral' | 'signal'
  icon?: IconName
  /** loading or empty states announce themselves; errors are alerts */
  role?: 'status' | 'alert'
  className?: string
}

/**
 * An empty, error or setup state inside a page that already has its title: a square mark, an h2 and the words,
 * on paper. Unlike StatusMessage it sits in the page's flow, left aligned, under the page's own h1.
 */
export function Notice({ title, children, actions, tone = 'neutral', icon, role, className = '' }: NoticeProps) {
  const mark = icon ?? (tone === 'signal' ? 'alert' : 'idle')
  return (
    <div
      role={role}
      className={`flex max-w-[640px] gap-4 rounded-card bg-paper p-5 ${tone === 'signal' ? 'border-[1.5px] border-signal' : 'border border-line'} ${className}`}
    >
      <span
        aria-hidden="true"
        className={`grid size-10 flex-none place-items-center rounded-[2px] ${tone === 'signal' ? 'bg-signal text-on-signal' : 'border-[1.5px] border-ink text-ink'}`}
      >
        <Icon name={mark} size={20} />
      </span>
      <div className="flex min-w-0 flex-col gap-1.5">
        <h2 className="m-0 pt-1.5 text-lg leading-tight font-bold">{title}</h2>
        {children && <div className="leading-normal text-ink-2">{children}</div>}
        {actions && <div className="mt-2.5 flex flex-wrap gap-2.5">{actions}</div>}
      </div>
    </div>
  )
}
