import { NewItemDialog } from './NewItemDialog'

// Create dialogs and edit pages. Tasks (TaskForm) are wired to the API;
// features still run on mock data and aren't routed (AGENT_BRIEF §7).

export const NewFeatureDialog = () => <NewItemDialog kind="feature" />
export { FeatureEditPage } from './EditPage'
export { NewTaskDialog, TaskEditPage } from './TaskForm'
