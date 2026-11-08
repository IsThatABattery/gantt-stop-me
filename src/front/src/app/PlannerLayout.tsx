import { Link, Outlet, useLocation, useParams } from 'react-router'
import { Logo } from '../components/Logo'
import { StatusMessage } from '../components/StatusMessage'
import { Stepper } from '../components/Stepper'
import { buttonClass } from '../components/buttonClass'
import { NotFoundError, useProject } from '../data/core'
import { useDraft } from '../data/views/plannerDoc'
import { plannerLabels } from '../labels/labels'

const STEP_PATHS = ['briefing', 'doc', 'tasks', 'done']

/**
 * Shell for the four planner steps (PlanBriefing … PlanDone): logo,
 * breadcrumb, stepper and "Save draft and exit". No nav rail, as in the mockups.
 */
export function PlannerLayout() {
  const { projectId = '', draftId = '' } = useParams()
  const { pathname } = useLocation()
  const project = useProject(projectId)
  const draft = useDraft(draftId)
  const current = Math.max(0, STEP_PATHS.indexOf(pathname.split('/').pop() ?? ''))
  const base = `/projects/${projectId}/plan/${draftId}`
  const error = project.error ?? draft.error

  return (
    <div className="stars min-h-screen bg-background text-text">
      <header className="flex flex-wrap items-center gap-x-8 gap-y-4 border-b border-line-soft bg-rail px-8 py-3.5">
        <Link to={`/projects/${projectId}`} className="no-underline">
          <Logo />
        </Link>
        <div className="text-[13px] text-text-muted">
          {project.data?.name ?? '…'} / {plannerLabels.newFeature} /{' '}
          <span className="text-text">
            {draft.data?.name ?? '…'}
            {draft.data && draft.data.status !== 'approved' && ` ${plannerLabels.draftSuffix}`}
          </span>
        </div>
        <div className="mx-auto">
          <Stepper
            label={plannerLabels.stepsLabel}
            current={current}
            steps={plannerLabels.steps.map((label, i) => ({ label, to: `${base}/${STEP_PATHS[i]}` }))}
          />
        </div>
        <Link to={`/projects/${projectId}`} className={buttonClass('secondary')}>
          {plannerLabels.saveAndExit}
        </Link>
      </header>
      <main className="mx-auto max-w-[1320px] px-8 pt-7 pb-12">
        {error ? (
          <StatusMessage
            title={error instanceof NotFoundError ? error.message : 'Something went wrong'}
            body="Check the link, or go back to the board."
          />
        ) : (
          <Outlet />
        )}
      </main>
    </div>
  )
}
