import { Link } from 'react-router'
import { Chip } from '../../components/Chip'
import { Icon, type IconName } from '../../components/Icon'
import { runLook } from '../../components/runLook'
import type { Task } from '../../data/api/types'
import { labels, noTestRunLabel, runStatusLabels, taskStatusLabels } from '../../labels/labels'
import { needsPerson } from './filters'

/** A task whose tests have never run */
const NO_TEST_RUN = noTestRunLabel

interface TaskCardProps {
  task: Task
  /** Opens the task drawer */
  to: string
  /** The task sits in User Check, waiting for a person to approve it */
  awaitingApproval?: boolean
  /** The copy under the pointer while dragging: not focusable, hidden from screen readers */
  overlay?: boolean
}

type Tone = 'plain' | 'signal' | 'approve' | 'closed'

/** The outer edge, then the inner keyline frame, per tone. Hover and focus ink the outer edge on every tone. */
const edges: Record<Tone, { outer: string; inner: string }> = {
  plain: { outer: 'border-line bg-paper', inner: 'border-keyline p-2 group-hover:border-ink group-focus-visible:border-ink' },
  signal: { outer: 'border-signal bg-paper', inner: 'border-signal p-2 group-hover:border-signal-ink group-focus-visible:border-signal-ink' },
  approve: { outer: 'border-line bg-paper', inner: 'border-2 border-approve p-[7px]' },
  // A closed card recedes into its tile through colour, never opacity, so its text and focus edge keep their contrast.
  closed: { outer: 'border-line bg-paper/55', inner: 'border-tile-keyline p-2 group-hover:border-ink group-focus-visible:border-ink' },
}

/**
 * One GitHub issue on the board, as a white keylined card: its number, title, assignees and one state chip. A card a
 * person must look at takes the signal edge and says why; one awaiting approval takes the yellow inner frame.
 */
export function TaskCard({ task, to, awaitingApproval = false, overlay = false }: TaskCardProps) {
  const closed = task.state === 'closed'
  const tone: Tone = closed ? 'closed' : needsPerson(task) ? 'signal' : awaitingApproval ? 'approve' : 'plain'
  const status = [
    task.needsAttention && labels.needsAttention,
    closed && taskStatusLabels.landed,
    task.runStatus ? runStatusLabels[task.runStatus] : !closed && NO_TEST_RUN,
  ]
    .filter(Boolean)
    .join(', ')
  const assigned = task.assignees.length ? `. Assigned to ${task.assignees.map((a) => a.login).join(', ')}` : ''
  const reasonId = `task-${task.id}-reason`
  const edge = edges[tone]

  return (
    <Link
      to={to}
      data-task-id={overlay ? undefined : task.id}
      tabIndex={overlay ? -1 : undefined}
      aria-hidden={overlay || undefined}
      aria-label={`#${task.issueNumber}: ${task.title}. ${status}${assigned}`}
      aria-describedby={overlay ? undefined : task.needsAttention ? `${reasonId} board-keys` : 'board-keys'}
      // The board's own drag replaces the browser's native link drag.
      draggable={false}
      className={`group block rounded-card border p-[3px] text-ink no-underline transition-colors duration-150 ease-out-quint hover:border-ink hover:text-ink focus-visible:border-ink ${
        overlay ? 'rotate-[1.5deg] cursor-grabbing border-ink bg-paper' : edge.outer
      }`}
    >
      <div className={`flex flex-col gap-1.5 rounded-[2px] border transition-colors duration-150 ease-out-quint ${edge.inner}`}>
        <div className="flex min-h-[22px] items-center justify-between gap-2">
          <span className="font-mono text-[12.5px] font-semibold text-ink-2">#{task.issueNumber}</span>
          {task.assignees.length > 0 && (
            <span className="flex flex-none gap-[3px]" title={task.assignees.map((a) => a.login).join(', ')}>
              {task.assignees.slice(0, 3).map((a) => (
                <img key={a.login} src={a.avatarUrl} alt="" width={22} height={22} className="block size-[22px] rounded-[2px] bg-block" />
              ))}
              {task.assignees.length > 3 && (
                <span className="grid h-[22px] min-w-[22px] place-items-center rounded-[2px] bg-block px-[3px] text-[11px] font-bold text-ink">
                  +{task.assignees.length - 3}
                </span>
              )}
            </span>
          )}
        </div>
        <p className={`m-0 line-clamp-3 text-sm leading-[1.3] ${closed ? 'font-medium text-ink-2' : 'font-semibold'}`}>{task.title}</p>
        <div className="mt-0.5 flex flex-wrap gap-[5px]">
          <StateChip task={task} closed={closed} />
        </div>
        {task.needsAttention && (
          <p id={overlay ? undefined : reasonId} className="m-0 rounded-[2px] bg-signal-wash px-2 py-1.5 text-[12.5px] leading-[1.4] text-signal-ink">
            {task.needsAttention}
          </p>
        )}
      </div>
    </Link>
  )
}

/**
 * One state, always an icon and a word. Needs attention outranks the run that caused it; Closed says it all on a
 * closed card (the drawer keeps the last run). No run yet, and Closed, are quiet words with no fill.
 */
function StateChip({ task, closed }: { task: Task; closed: boolean }) {
  if (closed) return <QuietState icon="closed">{taskStatusLabels.landed}</QuietState>
  if (task.needsAttention)
    return (
      <Chip tone="signal" icon="alert">
        {labels.needsAttention}
      </Chip>
    )
  if (!task.runStatus) return <QuietState icon="idle">{NO_TEST_RUN}</QuietState>
  const run = runLook[task.runStatus]
  return (
    <Chip tone={run.tone} icon={run.icon} iconClassName={run.iconClass}>
      {runStatusLabels[task.runStatus]}
    </Chip>
  )
}

function QuietState({ icon, children }: { icon: IconName; children: string }) {
  return (
    <span className="inline-flex h-6 items-center gap-[5px] text-xs leading-none font-medium text-ink-2">
      <Icon name={icon} size={14} className="flex-none" />
      {children}
    </span>
  )
}
