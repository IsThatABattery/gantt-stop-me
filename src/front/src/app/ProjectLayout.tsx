import { Outlet, useParams } from 'react-router'
import { NotFoundError } from '../data/api'
import { useProject } from '../data/queries'
import type { Project } from '../data/types'
import { labels } from '../labels/labels'
import { StatusMessage } from '../components/StatusMessage'
import { AppShell } from './AppShell'

/** Shell for every /projects/:projectId page. Loads the project once. */
export function ProjectLayout() {
  const { projectId = '' } = useParams()
  const { data: project, error, isPending } = useProject(projectId)

  return (
    <AppShell projectId={projectId}>
      {isPending ? (
        <StatusMessage title={`Loading ${labels.project.toLowerCase()}…`} />
      ) : error ? (
        <StatusMessage
          title={error instanceof NotFoundError ? `${labels.project} not found` : 'Something went wrong'}
          body={error instanceof NotFoundError ? 'Check the link, or pick a mission from the list.' : error.message}
        />
      ) : (
        <Outlet context={project satisfies Project} />
      )}
    </AppShell>
  )
}
