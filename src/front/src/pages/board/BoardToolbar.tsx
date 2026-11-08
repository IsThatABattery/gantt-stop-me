import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { labels } from '../../labels/labels'

interface BoardToolbarProps {
  projectId: number
  /** Shown when a counter filter is on */
  onClear?: () => void
}

const kbd = 'rounded border border-line bg-surface px-1.5 font-mono text-[11px] text-text-muted-strong'

// "New payload" opens the create dialog over the board.
export function BoardToolbar({ projectId, onClear }: BoardToolbarProps) {
  return (
    <div className="flex flex-col gap-2 pb-4">
      <div className="flex flex-wrap items-center gap-3">
        {onClear && (
          <button type="button" onClick={onClear} className={buttonClass('link', 'px-3')}>
            Clear filters
          </button>
        )}
        <Link to={`/projects/${projectId}/new/task`} className={buttonClass('accent', 'ml-auto')}>
          {labels.newTask}
        </Link>
      </div>
      <p id="board-keys" className="m-0 text-xs text-text-muted">
        <kbd className={kbd}>←</kbd> <kbd className={kbd}>↑</kbd> <kbd className={kbd}>↓</kbd>{' '}
        <kbd className={kbd}>→</kbd> move between {labels.tasks.toLowerCase()} · <kbd className={kbd}>[</kbd>{' '}
        <kbd className={kbd}>]</kbd> move one column left or right · <kbd className={kbd}>Enter</kbd> opens it
      </p>
    </div>
  )
}
