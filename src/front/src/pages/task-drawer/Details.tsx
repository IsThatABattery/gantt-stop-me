import type { TaskDetail } from '../../data/api/types'
import { formLabels } from '../../labels/labels'
import { Assignees } from './Assignees'
import { Card } from './Card'

const term = 'm-0 text-[13px] leading-6 font-semibold text-ink-2'

/** The Details tab: who it is assigned to, its branch and labels, then the issue's description. */
export function Details({ projectId, task }: { projectId: string; task: TaskDetail }) {
  // board:* labels repeat the column, which the tab already shows.
  const shown = task.labels.filter((l) => !l.name.startsWith('board:'))
  const repo = task.url.replace(/\/issues\/\d+$/, '')

  return (
    <div className="flex flex-col gap-2.5 pt-3">
      <Card>
        <dl className="m-0 grid grid-cols-1 gap-x-4 gap-y-1.5 sm:grid-cols-[88px_minmax(0,1fr)] sm:gap-y-3.5">
          <dt className={term}>Assignees</dt>
          <dd className="m-0 min-w-0 max-sm:mb-2">
            <Assignees projectId={projectId} task={task} />
          </dd>
          <dt className={term}>Branch</dt>
          <dd className="m-0 min-w-0 text-[13px] leading-6 max-sm:mb-2">
            {task.branch ? (
              <a href={`${repo}/tree/${task.branch}`} target="_blank" rel="noreferrer" className="font-mono text-[13px] font-medium break-all">
                {task.branch}
              </a>
            ) : (
              <span className="text-ink-2">Made on GitHub when the task moves to In Progress.</span>
            )}
          </dd>
          {shown.length > 0 && (
            <>
              <dt className={term}>Labels</dt>
              <dd className="m-0 min-w-0">
                <ul className="m-0 flex list-none flex-wrap gap-1.5 p-0">
                  {shown.map((l) => (
                    <li key={l.name} className="inline-flex h-6 items-center gap-1.5 rounded-[2px] bg-block px-2 text-xs font-semibold text-ink">
                      {/* The label's own colour, from GitHub; the ring keeps a pale one visible on the block. */}
                      <span className="size-2.5 flex-none rounded-[1px] ring-1 ring-ink/25" style={{ background: `#${l.color}` }} aria-hidden="true" />
                      {l.name}
                    </li>
                  ))}
                </ul>
              </dd>
            </>
          )}
        </dl>
      </Card>

      <Card>
        <h2 className="m-0 text-[13px] font-semibold text-ink-2">{formLabels.description}</h2>
        {task.body.trim() ? (
          // Issue bodies are Markdown; shown as written until there's a renderer.
          <p className="m-0 max-w-[68ch] text-sm leading-relaxed break-words whitespace-pre-wrap text-ink">{task.body}</p>
        ) : (
          <p className="m-0 text-[13px] leading-normal text-ink-2">No description yet.</p>
        )}
      </Card>
    </div>
  )
}
