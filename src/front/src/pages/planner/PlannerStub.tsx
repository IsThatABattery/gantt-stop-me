import { NotBuilt } from '../../components/NotBuilt'
import { plannerLabels } from '../../labels/labels'

/** Body for a planner step that its branch has not built yet. */
export function PlannerStub({ step, branch, doc }: { step: number; branch: string; doc: string }) {
  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="m-0 font-heading text-[26px] font-semibold">{plannerLabels.steps[step]}</h1>
      </div>
      <NotBuilt branch={branch} doc={doc} />
    </div>
  )
}
