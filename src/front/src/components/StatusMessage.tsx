import type { ReactNode } from 'react'

/**
 * Centered message for loading, empty, error and not-built states. An error
 * state is a dead end without a way out, so pass a Try again button
 * (refetch) as children, and show it only while there is no data: a failed
 * background refetch keeps the content that already loaded.
 */
export function StatusMessage({ title, body, children }: { title: string; body?: ReactNode; children?: ReactNode }) {
  return (
    <div role="status" className="flex flex-1 flex-col items-center justify-center gap-3 px-8 py-24 text-center">
      <h1 className="m-0 font-heading text-xl font-semibold">{title}</h1>
      {body && <p className="m-0 max-w-md leading-normal wrap-anywhere text-text-muted-strong">{body}</p>}
      {children}
    </div>
  )
}
