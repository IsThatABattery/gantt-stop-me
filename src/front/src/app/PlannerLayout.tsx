import { Link, Outlet, useLocation, useParams } from 'react-router'
import { Logo } from '../components/Logo'
import { StatusMessage } from '../components/StatusMessage'
import { Stepper } from '../components/Stepper'
import { buttonClass } from '../components/buttonClass'
import { NotFoundError, useMockProject } from '../data/core'
import { useDraft } from '../data/views/plannerDoc'
import { plannerLabels } from '../labels/labels'

const STEP_PATHS = ['briefing', 'doc', 'tasks', 'done']

/**
 * Shell for the four planner steps (PlanBriefing … PlanDone): logo,
 * breadcrumb, stepper and "Save draft and exit", on the rail colour. No nav rail, as in the mockups.
 */
export function PlannerLayout() {
  const { projectId = '', draftId = '' } = useParams()
  const { pathname } = useLocation()
  const project = useMockProject(projectId)
  const draft = useDraft(draftId)
  const current = Math.max(0, STEP_PATHS.indexOf(pathname.split('/').pop() ?? ''))
  const base = `/projects/${projectId}/plan/${draftId}`
  const error = project.error ?? draft.error

  return (
    <div className="stars min-h-screen bg-ground text-ink">
      <header className="flex flex-wrap items-center gap-x-8 gap-y-3 border-b border-rule bg-rail px-4 py-2.5 md:px-8">
        <Link to={`/projects/${projectId}`} className="flex min-h-11 items-center no-underline">
          <Logo />
        </Link>
        <div className="text-[13px] text-ink-2">
          {project.data?.name ?? '…'} / {plannerLabels.newFeature} /{' '}
          <span className="font-semibold text-ink">
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
      <main className="mx-auto max-w-[1320px] px-4 pt-7 pb-12 md:px-8">
        {error ? (
          <StatusMessage
            tone="signal"
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
