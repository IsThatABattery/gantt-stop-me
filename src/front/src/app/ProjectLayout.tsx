import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link, Outlet, useParams } from 'react-router'
import { buttonClass } from '../components/buttonClass'
import { StatusMessage } from '../components/StatusMessage'
import { useProject } from '../data/core'
import type { Project } from '../data/api/types'
import { errorCode, NotFoundError } from '../data/errors'
import { keys } from '../data/keys'
import { labels } from '../labels/labels'
import { AppShell } from './AppShell'

/** Shell for every /projects/:projectId page. Loads the project once, with its board. */
export function ProjectLayout() {
  const { projectId = '' } = useParams()
  const { data: project, error, isPending, dataUpdatedAt } = useProject(projectId)
  const queryClient = useQueryClient()
  // Each board load is a visit, so the rail reorders, and it follows a move, edit or sync, so assignments and
  // stages may have changed. dataUpdatedAt moves only once the board answered, after the visit was recorded.
  useEffect(() => {
    if (!dataUpdatedAt) return
    void queryClient.invalidateQueries({ queryKey: keys.projects })
    void queryClient.invalidateQueries({ queryKey: keys.assigned })
  }, [dataUpdatedAt, queryClient])

  return (
    <AppShell projectId={projectId}>
      {isPending ? (
        <StatusMessage title={`Loading ${labels.project.toLowerCase()}…`} />
      ) : errorCode(error) === 'github_not_linked' ? (
        <StatusMessage title="Link GitHub first" body={`${labels.projects} are GitHub repositories, so the API needs your GitHub account to open them.`}>
          <Link to="/settings#github" className={buttonClass('primary')}>
            {labels.linkGitHub}
          </Link>
        </StatusMessage>
      ) : error ? (
        <StatusMessage
          title={error instanceof NotFoundError ? `${labels.project} not found` : 'Something went wrong'}
          body={
            error instanceof NotFoundError
              ? `It may not exist, or your GitHub account can't see its repository. Add it again by its URL.`
              : error.message
          }
        >
          {error instanceof NotFoundError && (
            <Link to="/" className={buttonClass('secondary')}>
              {labels.addProject}
            </Link>
          )}
        </StatusMessage>
      ) : (
        <Outlet context={project satisfies Project} />
      )}
    </AppShell>
  )
}
