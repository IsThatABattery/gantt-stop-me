import { NewItemDialog } from './NewItemDialog'

// Create dialogs and edit pages for objectives and payloads. One shared form,
// parameterised by kind (AGENT_BRIEF §7).

export const NewFeatureDialog = () => <NewItemDialog kind="feature" />
export const NewTaskDialog = () => <NewItemDialog kind="task" />
export { FeatureEditPage, TaskEditPage } from './EditPage'
