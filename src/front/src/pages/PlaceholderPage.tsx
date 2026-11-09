import { NotBuilt } from '../components/NotBuilt'
import { PageTitle } from '../components/PageTitle'
import { Notice } from './projects/Notice'

interface PlaceholderPageProps {
  title: string
  /** Branch and scope doc that will build it; omit for screens with no branch yet */
  branch?: string
  doc?: string
}

/** Stand-in for screens not built yet. Lives inside the app shell, with the page header every screen has. */
export function PlaceholderPage({ title, branch, doc }: PlaceholderPageProps) {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-6 px-4 pt-5 pb-10 md:px-7">
      <header>
        <PageTitle>{title}</PageTitle>
      </header>
      {branch && doc ? (
        <NotBuilt branch={branch} doc={doc} />
      ) : (
        <Notice role="status" title="Not built yet">
          This screen needs a design before it gets a branch.
        </Notice>
      )}
    </div>
  )
}
