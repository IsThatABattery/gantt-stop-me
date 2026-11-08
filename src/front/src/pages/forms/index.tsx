import { NewItemDialog } from './NewItemDialog'

// Create dialogs and edit pages. Payloads (TaskForm) are wired to the API;
// objectives still run on mock data and aren't routed (AGENT_BRIEF §7).

export const NewFeatureDialog = () => <NewItemDialog kind="feature" />
export { FeatureEditPage } from './EditPage'
export { NewTaskDialog, TaskEditPage } from './TaskForm'
