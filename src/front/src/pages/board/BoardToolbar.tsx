import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Icon } from '../../components/Icon'
import { labels } from '../../labels/labels'

interface BoardToolbarProps {
  projectId: number
  /** Read-only members get no create links and no move keys */
  canWrite: boolean
  /** Shown when a counter filter is on */
  onClear?: () => void
}

// "New payload" opens the create dialog over the board.
export function BoardToolbar({ projectId, canWrite, onClear }: BoardToolbarProps) {
  return (
    <div className="flex flex-col pb-4">
      <div className="flex flex-wrap items-center gap-3">
        {/* Sits above Backlog, the first column, where planned tasks land. */}
        {canWrite && (
          <Link to={`/projects/${projectId}/plans`} className={buttonClass('primary', 'gap-2')}>
            <Icon name="target" size={16} />
            {labels.plans}
          </Link>
        )}
        {onClear && (
          <button type="button" onClick={onClear} className={buttonClass('link', 'px-3')}>
            Clear filters
          </button>
        )}
        {canWrite && (
          <Link to={`/projects/${projectId}/new/task`} className={buttonClass('accent', 'ml-auto')}>
            {labels.newTask}
          </Link>
        )}
      </div>
      {/* Cards drag with the pointer; the keys stay for keyboard users, described to screen readers only. */}
      <p id="board-keys" className="sr-only">
        Arrow keys move between {labels.tasks.toLowerCase()}. {canWrite && '[ and ] move one column left or right. '}Enter opens it.
      </p>
    </div>
  )
}
