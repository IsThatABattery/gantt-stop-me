import { PlannerStub } from '../PlannerStub'

// Planner steps 3–4. Stubs: feature/frontend-planner-tasks replaces them.
const BRANCH = 'feature/frontend-planner-tasks'
export const TaskReviewPage = () => <PlannerStub step={2} branch={BRANCH} doc="planner-tasks.md" />
export const PlanDonePage = () => <PlannerStub step={3} branch={BRANCH} doc="planner-tasks.md" />
