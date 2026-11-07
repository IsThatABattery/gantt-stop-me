import { PlannerStub } from '../PlannerStub'

// Planner steps 1–2. Stubs: feature/frontend-planner-doc replaces them.
const BRANCH = 'feature/frontend-planner-doc'
export const BriefingPage = () => <PlannerStub step={0} branch={BRANCH} doc="planner-doc.md" />
export const DocReviewPage = () => <PlannerStub step={1} branch={BRANCH} doc="planner-doc.md" />
