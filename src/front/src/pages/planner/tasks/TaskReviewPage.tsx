import { useState, type ReactNode } from 'react'
import { Link, useParams } from 'react-router'
import { StatusMessage } from '../../../components/StatusMessage'
import { buttonClass } from '../../../components/buttonClass'
import {
  isApproved,
  needsReview,
  usePlanSummary,
  useProposals,
  useReviewContext,
  type PlanSummary,
  type ReviewContext,
} from '../../../data/views/plannerTasks'
import type { TaskProposal } from '../../../data/types'
import { labels, plannerLabels } from '../../../labels/labels'
import { ProposalDetail } from './ProposalDetail'
import { ProposalQueue } from './ProposalQueue'

const agentWord = labels.agent.toLowerCase()
const tasksWord = labels.tasks.toLowerCase()

/** Step 3 (PlanPayloads and PlanPayloadsDeny): review each proposed payload. */
export function TaskReviewPage() {
  const { projectId = '', draftId = '' } = useParams()
  const proposals = useProposals(draftId)
  const context = useReviewContext(draftId)
  const summary = usePlanSummary(draftId)
  const base = `/projects/${projectId}/plan/${draftId}`

  if (proposals.error) return <StatusMessage title="Something went wrong" body={proposals.error.message} />
  if (!proposals.data) return <StatusMessage title="Loading…" />

  const list = proposals.data
  const header = (
    <div>
      <h1 className="m-0 font-heading text-[28px] font-semibold">Review proposed {tasksWord}</h1>
      <p className="mt-2 mb-0 max-w-[680px] leading-normal text-text-muted-strong">
        Approved {tasksWord} go to the {labels.backlog}. Denied ones go back to the planning {agentWord} with your feedback.
      </p>
    </div>
  )

  if (list.length === 0) {
    return (
      <div className="flex flex-col gap-6">
        {header}
        <StatusMessage
          title={`No ${tasksWord} proposed yet`}
          body={`The planning ${agentWord} has not proposed any ${tasksWord} from this doc yet. Check back soon.`}
        >
          <Link to={`/projects/${projectId}`} className={buttonClass('secondary')}>
            {plannerLabels.openBoard}
          </Link>
        </StatusMessage>
      </div>
    )
  }

  return (
    <Review key={draftId} list={list} context={context.data} summary={summary.data} base={base}>
      {header}
    </Review>
  )
}

interface ReviewProps {
  list: TaskProposal[]
  context?: ReviewContext
  summary?: PlanSummary
  base: string
  children: ReactNode
}

function Review({ list, context, summary, base, children }: ReviewProps) {
  // Start on the first proposal that still needs review, and stay on it after
  // approving so the result shows, until the operator picks another.
  const [picked, setPicked] = useState(() => (list.find(needsReview) ?? list[0]).id)
  const selected = list.find((p) => p.id === picked) ?? list[0]
  const left = list.filter(needsReview).length
  const reviewed = list.length - left
  const approved = list.filter(isApproved).length
  const sentBack = list.filter((p) => p.status === 'sent-back').length

  return (
    <div className="flex flex-col gap-6">
      {children}
      <div className="flex flex-wrap items-start gap-6">
        <ProposalQueue proposals={list} selectedId={selected.id} onSelect={setPicked} />
        <ProposalDetail proposal={selected} proposals={list} context={context} summary={summary} onSelect={setPicked} />
      </div>

      <div className="sticky bottom-0 -mx-8 -mb-12 flex flex-wrap items-center gap-4 border-t border-line-soft bg-rail px-8 py-4 backdrop-blur">
        <div className="flex-[1_1_280px]">
          <div role="status" className="text-[13px]">
            {reviewed} of {list.length} reviewed · {approved} approved · {sentBack} sent back
          </div>
          <div role="img" aria-label={`${reviewed} of ${list.length} reviewed`} className="mt-2 h-1.5 overflow-hidden rounded-[3px] bg-line-soft">
            <div className="h-1.5 bg-good" style={{ width: `${Math.round((reviewed / list.length) * 100)}%` }} />
          </div>
        </div>
        <Link to={`${base}/done`} className={buttonClass(left === 0 ? 'primary' : 'secondary')}>
          {plannerLabels.finishReview}
          {left > 0 && ` (${left} left)`}
        </Link>
      </div>
    </div>
  )
}
