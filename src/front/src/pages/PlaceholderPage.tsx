import { NotBuilt } from '../components/NotBuilt'

interface PlaceholderPageProps {
  title: string
  /** Branch and scope doc that will build it; omit for screens with no branch yet */
  branch?: string
  doc?: string
}

/** Stand-in for screens not built yet. Lives inside the app shell. */
export function PlaceholderPage({ title, branch, doc }: PlaceholderPageProps) {
  return (
    <div className="flex flex-col gap-6 px-8 py-7">
      <header>
        <h1 className="m-0 font-heading text-[26px] font-semibold">{title}</h1>
      </header>
      {branch && doc ? (
        <NotBuilt branch={branch} doc={doc} />
      ) : (
        <div
          role="status"
          className="max-w-xl rounded-[14px] border border-dashed border-line-strong bg-surface/60 p-6 leading-normal text-text-muted-strong"
        >
          <div className="mb-1 font-medium text-text">Not built yet</div>
          This screen needs a design before it gets a branch.
        </div>
      )}
    </div>
  )
}
