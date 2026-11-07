import { buttonClass } from '../../components/buttonClass'
import { Select } from '../../components/controls'
import type { Feature } from '../../data/types'
import { labels } from '../../labels/labels'

// Shown but inert in Phase 1: filtering and create forms come in Phase 2.
export function BoardToolbar({ features }: { features: Feature[] }) {
  return (
    <div className="flex flex-wrap items-center gap-3 pb-4">
      <label htmlFor="feature-filter" className="text-[13px] text-text-muted">
        {labels.feature}
      </label>
      <Select id="feature-filter" className="min-h-11 bg-surface">
        <option>{labels.allFeatures}</option>
        {[...features]
          .sort((a, b) => a.name.localeCompare(b.name))
          .map((f) => (
            <option key={f.id} value={f.id}>
              {f.name}
            </option>
          ))}
      </Select>
      <button type="button" className={buttonClass('link', 'px-3')}>
        {labels.manageFeatures}
      </button>
      <button type="button" className={buttonClass('accent')}>
        {labels.newFeature}
      </button>
      <button type="button" className={buttonClass('accent')}>
        {labels.newTask}
      </button>
    </div>
  )
}
