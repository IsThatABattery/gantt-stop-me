/** Stand-in body for a screen that a view branch will build. */
export function NotBuilt({ branch, doc }: { branch: string; doc: string }) {
  return (
    <div role="status" className="max-w-xl rounded-card border border-dashed border-line bg-paper p-6 leading-normal text-ink-2">
      <div className="mb-1 font-bold text-ink">Not built yet</div>
      Built on <code className="font-mono text-[13px] text-ink">{branch}</code>. Scope:{' '}
      <code className="font-mono text-[13px] text-ink">docs/implementation/{doc}</code>
    </div>
  )
}
