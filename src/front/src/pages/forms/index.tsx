import { useNavigate, useParams } from 'react-router'
import { Dialog } from '../../components/Dialog'
import { NotBuilt } from '../../components/NotBuilt'
import { formLabels } from '../../labels/labels'
import { PlaceholderPage } from '../PlaceholderPage'

// Create dialogs and edit pages for objectives and payloads.
// Stubs: feature/frontend-forms replaces them.

const BRANCH = 'feature/frontend-forms'
const DOC = 'forms.md'

function NewDialog({ kind }: { kind: 'feature' | 'task' }) {
  const { projectId } = useParams()
  const navigate = useNavigate()
  const t = formLabels[kind]
  return (
    <Dialog label={t.titleNew} onClose={() => navigate(`/projects/${projectId}`)}>
      <div className="flex flex-col gap-4 p-7 pr-16">
        <h2 className="m-0 font-heading text-[22px] font-semibold">{t.titleNew}</h2>
        <NotBuilt branch={BRANCH} doc={DOC} />
      </div>
    </Dialog>
  )
}

export const NewFeatureDialog = () => <NewDialog kind="feature" />
export const NewTaskDialog = () => <NewDialog kind="task" />
export const FeatureEditPage = () => <PlaceholderPage title={formLabels.feature.titleEdit} branch={BRANCH} doc={DOC} />
export const TaskEditPage = () => <PlaceholderPage title={formLabels.task.titleEdit} branch={BRANCH} doc={DOC} />
