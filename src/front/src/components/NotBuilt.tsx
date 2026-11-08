/** Stand-in body for a screen that a view branch will build. */
export function NotBuilt({ branch, doc }: { branch: string; doc: string }) {
  return (
    <div
      role="status"
      className="max-w-xl rounded-[14px] border border-dashed border-line-strong bg-surface/60 p-6 leading-normal text-text-muted-strong"
    >
      <div className="mb-1 font-medium text-text">Not built yet</div>
      Built on <code className="font-mono text-[13px] text-ion">{branch}</code>. Scope:{' '}
      <code className="font-mono text-[13px]">docs/implementation/{doc}</code>
    </div>
  )
}
