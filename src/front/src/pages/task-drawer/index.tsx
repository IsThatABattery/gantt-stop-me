import { useNavigate, useParams } from 'react-router'
import { Drawer } from '../../components/Drawer'
import { NotBuilt } from '../../components/NotBuilt'
import { labels } from '../../labels/labels'

// Payload drawer over the board. Stub: feature/frontend-task-drawer replaces it.
export function TaskDrawerPage() {
  const { projectId, taskId } = useParams()
  const navigate = useNavigate()
  return (
    <Drawer label={labels.task} onClose={() => navigate(`/projects/${projectId}`)}>
      <div className="flex flex-col gap-4 p-6 pr-16">
        <div className="eyebrow">
          {labels.task} · {taskId}
        </div>
        <NotBuilt branch="feature/frontend-task-drawer" doc="task-drawer.md" />
      </div>
    </Drawer>
  )
}
