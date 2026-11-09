import { Link } from 'react-router'
import { buttonClass } from '../../components/buttonClass'
import { ButtonMark } from '../../components/controls'
import { Counter } from '../../components/Counter'
import { Icon } from '../../components/Icon'
import { PageTitle } from '../../components/PageTitle'
import type { Project } from '../../data/api/types'
import { labels } from '../../labels/labels'
import { counters, taskCount, type CounterFilter } from './filters'

interface BoardHeaderProps {
  project: Project
  counts: Record<CounterFilter, number>
  /** Tasks on the board, and how many the filter shows */
  total: number
  shown: number
  /** The counter filtering the board; clicking it again clears it */
  active: CounterFilter | null
  onToggle: (filter: CounterFilter) => void
  onClear: () => void
}

/** Grows a small inline control's hit area to 44px tall without moving the line it sits in */
const hitArea = "relative before:absolute before:-inset-x-1 before:-inset-y-3 before:content-['']"

/**
 * The board's header, as in the process-flow sheet: the project name huge in display caps, the repository and task
 * count under it, then the three counters (each a filter), Plan a feature and the ink New task button.
 */
export function BoardHeader({ project, counts, total, shown, active, onToggle, onClear }: BoardHeaderProps) {
  const repo = `${project.owner}/${project.name}`
  return (
    // From 1400px the title and the actions fit on one row (the 224px rail, the gutters and B's header row), and the
    // repository line runs under both, so a filter changes only its words, never the layout. Narrower, it stacks:
    // title, repository line, then the actions.
    <header className="flex flex-col gap-y-4 min-[1400px]:flex-row min-[1400px]:flex-wrap min-[1400px]:items-end min-[1400px]:justify-between min-[1400px]:gap-x-8 min-[1400px]:gap-y-2.5">
      <PageTitle size="lg" className="min-w-0 min-[1400px]:order-1">
        {project.name}
      </PageTitle>
      <p className="m-0 -mt-1.5 flex flex-wrap items-center gap-x-1.5 gap-y-1 text-[15px] text-ink-2 min-[1400px]:order-3 min-[1400px]:mt-0 min-[1400px]:basis-full">
        <a
          href={`https://github.com/${repo}`}
          target="_blank"
          rel="noreferrer"
          className={`${hitArea} inline-flex items-center gap-1.5 font-semibold text-ink no-underline decoration-1 hover:text-ink hover:underline`}
        >
          <Icon name="github" size={15} className="flex-none" />
          {repo}
        </a>
        <span aria-hidden="true">·</span>
        <span aria-live="polite">{active ? `Showing ${taskCount(shown, total)}` : taskCount(total)}</span>
        {active && (
          <>
            <span aria-hidden="true">·</span>
            <button
              type="button"
              onClick={onClear}
              className={`${hitArea} cursor-pointer border-0 bg-transparent p-0 font-[inherit] text-[15px] font-semibold text-accent underline decoration-1 underline-offset-[3px] hover:text-ink hover:decoration-2`}
            >
              Clear filters
            </button>
          </>
        )}
      </p>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 min-[1400px]:order-2">
        <div role="group" aria-label="Filter the board" className="flex flex-wrap gap-2 lg:border-r lg:border-rule lg:pr-4">
          {counters.map((c) => (
            <Counter
              key={c.filter}
              tone={c.tone}
              count={counts[c.filter]}
              label={c.label}
              pressed={active === c.filter}
              onClick={() => onToggle(c.filter)}
            />
          ))}
        </div>
        <div className="flex flex-wrap gap-2">
          <Link to={`/projects/${project.id}/plans`} className={buttonClass('secondary', 'min-h-[52px]')}>
            <Icon name="plan" size={16} className="flex-none" />
            {labels.plans}
          </Link>
          <Link to={`/projects/${project.id}/new/task`} className={buttonClass('primary', 'min-h-[52px] pl-2 pr-[18px]')}>
            <ButtonMark icon="plus" />
            {labels.newTask}
          </Link>
        </div>
      </div>

      {/* Cards drag with the pointer; the keys stay for keyboard users, described to screen readers on every card. */}
      <p id="board-keys" className="sr-only">
        Arrow keys move between {labels.tasks.toLowerCase()}. [ and ] move one column left or right. Enter opens it.
      </p>
    </header>
  )
}
