import { Link } from 'react-router'
import { Chip } from '../../components/Chip'
import { Checkbox } from '../../components/controls'
import { Icon } from '../../components/Icon'
import { ProgressBar } from '../../components/ProgressBar'
import { Tabs } from '../../components/Tabs'
import type { ActivityEvent, Agent, Assignee, Hail, PullRequest, User } from '../../data/types'
import { useSetSubtaskDone, type TaskDetail } from '../../data/views/taskDrawer'
import { activityVerbLabels, formLabels, labels, priorityLabels, taskTabLabels } from '../../labels/labels'
import { clockTime } from './look'

export function TaskTabs({ detail }: { detail: TaskDetail }) {
  const { done, total } = detail.task.subtasks
  return (
    <Tabs
      label={`${labels.task} sections`}
      items={[
        { id: 'brief', label: taskTabLabels.brief, content: <BriefTab detail={detail} /> },
        {
          id: 'modules',
          label: total > 0 ? `${taskTabLabels.subtasks} ${done}/${total}` : taskTabLabels.subtasks,
          content: <ModulesTab detail={detail} />,
        },
        { id: 'log', label: taskTabLabels.log, content: <FlightLogTab detail={detail} /> },
        { id: 'cargo', label: taskTabLabels.cargo, content: <CargoTab pr={detail.pullRequest} /> },
      ]}
    />
  )
}

const panel = 'flex flex-col gap-4 px-6 py-5'
const heading = 'text-xs font-semibold text-ink-2 m-0'
const empty = 'm-0 text-[13px] leading-normal text-ink-2'

// ---- Brief ----

function BriefTab({ detail }: { detail: TaskDetail }) {
  const { task } = detail
  const met = task.acceptanceCriteria.filter((c) => c.met).length
  return (
    <div className={panel}>
      <section className="flex flex-col gap-2">
        <h2 className={heading}>{formLabels.description}</h2>
        {task.description ? (
          <p className="m-0 leading-normal text-ink">{task.description}</p>
        ) : (
          <p className={empty}>No description yet.</p>
        )}
      </section>

      <section className="flex flex-col gap-2">
        <div className="flex items-baseline justify-between">
          <h2 className={heading}>{formLabels.task.criteriaHeading}</h2>
          {task.acceptanceCriteria.length > 0 && (
            <span className="font-mono text-xs text-ink-2">
              {met} of {task.acceptanceCriteria.length} met
            </span>
          )}
        </div>
        {task.acceptanceCriteria.length === 0 ? (
          <p className={empty}>No acceptance criteria yet.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-2 p-0">
            {task.acceptanceCriteria.map((c) => (
              <li key={c.id} className="flex items-start gap-2.5 text-[13px] leading-normal">
                {c.met ? (
                  <Icon name="check" size={16} className="mt-0.5 flex-none text-good" />
                ) : (
                  <span className="mt-[3px] size-3.5 flex-none rounded-[2px] border-[1.5px] border-ink-2" aria-hidden="true" />
                )}
                <span className={c.met ? 'text-ink' : 'text-ink'}>
                  <span className="sr-only">{c.met ? 'Met: ' : 'Not met: '}</span>
                  {c.text}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <dl className="m-0 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-[13px]">
        <dt className="text-ink-2">{formLabels.priority}</dt>
        <dd className="m-0">{priorityLabels[task.priority]}</dd>
        <dt className="text-ink-2">{formLabels.tags}</dt>
        <dd className="m-0 flex flex-wrap gap-1.5">
          {task.tags.length === 0 ? <span className="text-ink-2">None</span> : task.tags.map((t) => <Chip key={t} tone="neutral" shape="tag">{t}</Chip>)}
        </dd>
      </dl>
    </div>
  )
}

// ---- Modules ----

function ModulesTab({ detail }: { detail: TaskDetail }) {
  const { task, subtasks } = detail
  const toggle = useSetSubtaskDone(task.projectId, task.id)
  const pendingId = toggle.isPending ? toggle.variables?.subtaskId : undefined

  if (subtasks.length === 0) {
    return (
      <div className={panel}>
        <p className={empty}>
          No {labels.subtask.toLowerCase()}s yet. Add them from the{' '}
          <Link to={`/projects/${task.projectId}/tasks/${task.id}/edit`}>edit page</Link>.
        </p>
      </div>
    )
  }

  return (
    <div className={panel}>
      <ProgressBar label={formLabels.task.progressLabel} value={task.subtasks.done} max={task.subtasks.total} />
      <ul className="m-0 flex list-none flex-col p-0">
        {subtasks.map((s) => (
          <li key={s.id} className="flex items-center justify-between gap-3 border-t border-keyline first:border-t-0">
            <Checkbox
              label={<span className={s.done ? 'text-ink-2 line-through' : 'text-ink'}>{s.title}</span>}
              checked={s.done}
              disabled={pendingId === s.id}
              onChange={(e) => toggle.mutate({ projectId: task.projectId, taskId: task.id, subtaskId: s.id, done: e.target.checked })}
            />
            <span className="text-xs whitespace-nowrap text-ink-2">{ownerName(s.assignee, detail)}</span>
          </li>
        ))}
      </ul>
      {toggle.error && (
        <p role="alert" className="m-0 text-[13px] text-signal-ink">
          {toggle.error.message}
        </p>
      )}
    </div>
  )
}

function ownerName(a: Assignee | null, { users, agents }: Pick<TaskDetail, 'users' | 'agents'>) {
  if (!a) return labels.unassigned
  return (a.kind === 'user' ? users.find((u) => u.id === a.id) : agents.find((g) => g.id === a.id))?.name ?? labels.unassigned
}

// ---- Log ----

type LogEntry = { kind: 'event'; at: string; event: ActivityEvent } | { kind: 'hail'; at: string; hail: Hail }

function FlightLogTab({ detail }: { detail: TaskDetail }) {
  const { task, events, hails, users, agents } = detail
  const open = hails.filter((h) => !h.answer)
  const entries: LogEntry[] = [
    ...events.map((event) => ({ kind: 'event' as const, at: event.at, event })),
    ...hails.filter((h) => h.answer && h.answeredAt).map((hail) => ({ kind: 'hail' as const, at: hail.answeredAt!, hail })),
  ].sort((a, b) => b.at.localeCompare(a.at))

  return (
    <div className={panel}>
      {open.map((h) => (
        <div key={h.id} className="flex flex-col gap-2 rounded-control border border-approve bg-approve/6 p-3.5">
          <Chip tone="warning" className="self-start">
            <Icon name="chat" size={12} />
            {labels.hail} · {labels.waitingOnYou}
          </Chip>
          <p className="m-0 text-[13px] leading-normal">{h.question}</p>
          <Link to={`/projects/${task.projectId}/agents`} className="text-[13px]">
            Answer in the {labels.agents} console
          </Link>
        </div>
      ))}
      {entries.length === 0 && open.length === 0 ? (
        <p className={empty}>Nothing logged for this {labels.task.toLowerCase()} yet.</p>
      ) : (
        <ol className="m-0 flex list-none flex-col p-0">
          {entries.map((e) =>
            e.kind === 'event' ? (
              <li key={e.event.id} className="flex gap-3 border-t border-keyline py-3 text-[13px] leading-normal first:border-t-0 first:pt-0">
                <time dateTime={e.at} className="w-11 flex-none font-mono text-xs leading-5 text-ink-2">
                  {clockTime(e.at)}
                </time>
                <span>
                  <Actor actor={e.event.actor} users={users} agents={agents} /> {activityVerbLabels[e.event.verb]} {task.key}
                  {e.event.detail && <span className="block text-ink-2">{e.event.detail}</span>}
                </span>
              </li>
            ) : (
              <li key={e.hail.id} className="flex gap-3 border-t border-keyline py-3 text-[13px] leading-normal first:border-t-0 first:pt-0">
                <time dateTime={e.at} className="w-11 flex-none font-mono text-xs leading-5 text-ink-2">
                  {clockTime(e.at)}
                </time>
                <span className="flex min-w-0 flex-col gap-1">
                  <span>
                    <Actor actor={{ kind: 'agent', id: e.hail.agentId }} users={users} agents={agents} /> asked:{' '}
                    {e.hail.question}
                  </span>
                  <span className="text-ink-2">
                    <Actor actor={{ kind: 'user', id: e.hail.answeredBy ?? '' }} users={users} agents={agents} /> answered: {e.hail.answer}
                  </span>
                </span>
              </li>
            ),
          )}
        </ol>
      )}
    </div>
  )
}

/** An agent's name with the agent mark, a person's plain, both in the accent. */
function Actor({ actor, users, agents }: { actor: Assignee; users: User[]; agents: Agent[] }) {
  if (actor.kind === 'agent') {
    const name = agents.find((a) => a.id === actor.id)?.name ?? labels.agent
    return (
      <span className="inline-flex items-center gap-1 font-medium text-accent">
        <Icon name="agent" size={14} className="text-accent" />
        <span className="sr-only">{labels.agent} </span>
        {name}
      </span>
    )
  }
  const name = users.find((u) => u.id === actor.id)?.name ?? labels.user
  return (
    <span className="font-medium text-accent">
      <span className="sr-only">{labels.user} </span>
      {name}
    </span>
  )
}

// ---- Files ----

const checksLook = {
  passing: { tone: 'good', icon: 'check', text: 'all checks passing' },
  failing: { tone: 'critical', icon: 'alert', text: 'checks failing' },
  pending: { tone: 'live', icon: 'running', text: 'checks running' },
} as const

function CargoTab({ pr }: { pr: PullRequest | null }) {
  if (!pr) {
    return (
      <div className={panel}>
        <p className={empty}>No files yet. A linked pull request shows here.</p>
      </div>
    )
  }
  const checks = checksLook[pr.checks]
  return (
    <div className={panel}>
      <div className="flex flex-col gap-2 rounded-control border border-rule bg-ground p-3.5">
        <div className="flex items-center justify-between gap-3 text-[13px]">
          <span className="inline-flex min-w-0 items-center gap-2">
            <Icon name="pr" size={16} className="flex-none text-accent" />
            <span className="font-medium">PR #{pr.number}</span>
            <span className="truncate font-mono text-xs text-ink-2">{pr.branch}</span>
          </span>
          <span className="font-mono text-xs whitespace-nowrap">
            <span className="text-good">+{pr.additions}</span> <span className="text-signal-ink">−{pr.deletions}</span>
            <span className="sr-only"> lines</span>
          </span>
        </div>
        <div className="flex flex-wrap items-center gap-2 text-[13px] text-ink-2">
          {pr.filesChanged} files changed ·
          <Chip tone={checks.tone}>
            <Icon name={checks.icon} size={12} />
            {checks.text}
          </Chip>
        </div>
      </div>
    </div>
  )
}
