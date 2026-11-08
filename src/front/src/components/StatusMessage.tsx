import type { ReactNode } from 'react'

/** Centered message for loading, empty, error and not-built states. */
export function StatusMessage({ title, body, children }: { title: string; body?: ReactNode; children?: ReactNode }) {
  return (
    <div role="status" className="flex flex-1 flex-col items-center justify-center gap-3 px-8 py-24 text-center">
      <h1 className="m-0 font-heading text-xl font-semibold">{title}</h1>
      {body && <p className="m-0 max-w-md leading-normal text-text-muted-strong">{body}</p>}
      {children}
    </div>
  )
}
