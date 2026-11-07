import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { Select } from '../../components/controls'
import type { Feature } from '../../data/types'
import { labels } from '../../labels/labels'

// The objective filter is inert until feature/frontend-board. The links open
// the create dialogs and the objective editor (feature/frontend-forms).
export function BoardToolbar({ projectId, features }: { projectId: string; features: Feature[] }) {
  const sorted = [...features].sort((a, b) => a.name.localeCompare(b.name))
  const base = `/projects/${projectId}`
  return (
    <div className="flex flex-wrap items-center gap-3 pb-4">
      <label htmlFor="feature-filter" className="text-[13px] text-text-muted">
        {labels.feature}
      </label>
      <Select id="feature-filter" className="min-h-11 bg-surface">
        <option>{labels.allFeatures}</option>
        {sorted.map((f) => (
          <option key={f.id} value={f.id}>
            {f.name}
          </option>
        ))}
      </Select>
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
  )
}
