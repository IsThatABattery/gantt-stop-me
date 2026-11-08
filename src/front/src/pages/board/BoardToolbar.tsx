import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Select } from '../../components/controls'
import type { Feature, ID } from '../../data/types'
import { labels } from '../../labels/labels'

interface BoardToolbarProps {
  projectId: ID
  features: Feature[]
  /** The objective filter; null shows all objectives */
  featureId: ID | null
  onFeatureChange: (featureId: ID | null) => void
  /** Shown when any filter is on */
  onClear?: () => void
}

const kbd = 'rounded border border-line bg-surface px-1.5 font-mono text-[11px] text-text-muted-strong'

// The objective filter narrows the cards to one feature. The links open the
// create dialogs and the objective editor (feature/frontend-forms).
export function BoardToolbar({ projectId, features, featureId, onFeatureChange, onClear }: BoardToolbarProps) {
  const sorted = [...features].sort((a, b) => a.name.localeCompare(b.name))
  const base = `/projects/${projectId}`
  return (
    <div className="flex flex-col gap-2 pb-4">
      <div className="flex flex-wrap items-center gap-3">
        <label htmlFor="feature-filter" className="text-[13px] text-text-muted">
          {labels.feature}
        </label>
        <Select
          id="feature-filter"
          className="min-h-11 bg-surface"
          value={featureId ?? ''}
          onChange={(e) => onFeatureChange(e.target.value || null)}
        >
          <option value="">{labels.allFeatures}</option>
          {sorted.map((f) => (
            <option key={f.id} value={f.id}>
              {f.name}
            </option>
          ))}
        </Select>
        {onClear && (
          <button type="button" onClick={onClear} className={buttonClass('link', 'px-3')}>
            Clear filters
          </button>
        )}
        {sorted[0] && (
          <Link to={`${base}/features/${sorted[0].id}/edit`} className={buttonClass('link', 'px-3')}>
            {labels.manageFeatures}
          </Link>
        )}
        <Link to={`${base}/new/feature`} className={buttonClass('accent')}>
          {labels.newFeature}
        </Link>
        <Link to={`${base}/new/task`} className={buttonClass('accent')}>
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
