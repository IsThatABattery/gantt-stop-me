import { Link, useParams } from 'react-router'
import { Chip } from '../../../components/Chip'
import { StatusMessage } from '../../../components/StatusMessage'
import { buttonClass } from '../../../components/buttonClass'
import { useDraft } from '../../../data/views/plannerDoc'
import { usePlanSummary } from '../../../data/views/plannerTasks'
import { columnLabels, labels, plannerLabels } from '../../../labels/labels'

const agentWord = labels.agent.toLowerCase()

/** Step 4 (PlanDone): what the review added to the board. */
export function PlanDonePage() {
  const { projectId = '', draftId = '' } = useParams()
  const { data: draft } = useDraft(draftId)
  const { data: summary, error } = usePlanSummary(draftId)
  const base = `/projects/${projectId}/plan/${draftId}`

  if (error) return <StatusMessage title="Something went wrong" body={error.message} />
  if (!summary || !draft) return <StatusMessage title="Loading…" />

  const tiles = [
    { n: summary.approved.length, text: `approved, now in the ${labels.backlog}`, tone: 'border-good bg-good/6' },
    { n: summary.sentBack, text: `sent back, the ${agentWord} is revising`, tone: 'border-warning bg-warning/6' },
    { n: summary.waiting, text: 'still waiting for your review', tone: 'border-line' },
    ...(summary.dropped > 0 ? [{ n: summary.dropped, text: 'dropped from the plan', tone: 'border-line' }] : []),
  ]

  return (
    <div className="mx-auto flex max-w-[920px] flex-col gap-6 pt-5">
      <div>
        <div className="eyebrow">STEP 4 · {plannerLabels.steps[3].toUpperCase()}</div>
        <h1 className="mt-1.5 mb-0 font-heading text-[28px] font-semibold">{draft.name} is ready</h1>
        <p className="mt-2 mb-0 leading-normal text-text-muted-strong">
          The {labels.feature.toLowerCase()} doc is approved and the reviewed {labels.tasks.toLowerCase()} are on the board.
        </p>
      </div>

      <ul aria-label="Summary" className="m-0 grid list-none grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4 p-0">
        {tiles.map((t) => (
          <li key={t.text} className={`rounded-card border p-[18px] ${t.tone}`}>
            <div className="font-mono text-2xl">{t.n}</div>
            <div className="mt-1 text-text-soft">{t.text}</div>
          </li>
        ))}
      </ul>

      <section aria-labelledby="added-h" className="rounded-[14px] border border-line bg-surface p-6">
        <h2 id="added-h" className="mt-0 mb-1 font-heading text-lg font-semibold">
          Added to the board
        </h2>
        {summary.approved.length === 0 ? (
          <p className="mt-2 mb-0 text-text-muted-strong">Nothing yet. Approve a proposal to add it to the {labels.backlog}.</p>
        ) : (
          <ul className="m-0 list-none p-0">
            {summary.approved.map((a) => (
              <li key={a.taskId} className="flex flex-wrap items-center justify-between gap-3 border-t border-line-soft py-3.5 first:border-t-0">
                <Link to={`/projects/${projectId}/tasks/${a.taskId}`} className="text-text no-underline hover:text-text">
                  <span className="font-mono text-xs text-text-muted">{a.key}</span>
                  <span className="mt-0.5 block hover:underline">{a.title}</span>
                </Link>
                <Chip tone="neutral">{a.column === 'backlog' ? `In the ${labels.backlog}` : columnLabels[a.column].name}</Chip>
              </li>
            ))}
          </ul>
        )}
      </section>

      <div className="flex flex-wrap gap-3">
        <Link to={`/projects/${projectId}`} className={buttonClass('primary')}>
          {plannerLabels.openBoard}
        </Link>
        <Link to={`${base}/tasks`} className={buttonClass('secondary')}>
          {plannerLabels.keepReviewing}
        </Link>
        {draft.featureId && (
          <Link to={`/projects/${projectId}/features/${draft.featureId}/edit`} className={buttonClass('secondary')}>
            {plannerLabels.viewFeature}
          </Link>
        )}
      </div>
    </div>
  )
}
