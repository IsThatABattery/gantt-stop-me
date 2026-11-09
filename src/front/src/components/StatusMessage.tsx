import type { ReactNode } from 'react'
import { Icon, type IconName } from './Icon'

interface StatusMessageProps {
  title: string
  body?: ReactNode
  children?: ReactNode
  /** A square mark above the title: signal (with the alert icon unless one is given) for errors, else a hollow one */
  tone?: 'neutral' | 'signal'
  icon?: IconName
}

/** Centered message for loading, empty, error and not-built states. */
export function StatusMessage({ title, body, children, tone = 'neutral', icon }: StatusMessageProps) {
  const mark = icon ?? (tone === 'signal' ? 'alert' : undefined)
  return (
    <div role="status" className="flex flex-1 flex-col items-center justify-center gap-3 px-6 py-24 text-center">
      {mark && (
        <span
          aria-hidden="true"
          className={`mb-1 grid size-10 place-items-center rounded-[2px] ${tone === 'signal' ? 'bg-signal text-on-signal' : 'border-[1.5px] border-ink text-ink'}`}
        >
          <Icon name={mark} size={20} />
        </span>
      )}
      <h1 className="m-0 text-xl font-bold">{title}</h1>
      {body && <p className="m-0 max-w-md leading-normal text-ink-2">{body}</p>}
      {children && <div className="mt-2 flex flex-wrap justify-center gap-2.5">{children}</div>}
    </div>
  )
}
