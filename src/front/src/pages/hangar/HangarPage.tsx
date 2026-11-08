import { Link, Outlet } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { AgentTag, UserAvatar } from '../../components/Avatar'
import { Chip } from '../../components/Chip'
import { Eyebrow } from '../../components/Eyebrow'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { useBoard } from '../../data/views/board'
import type { Board, Task } from '../../data/types'
import { labels, priorityLabels, taskStatusLabels } from '../../labels/labels'
import { cardLook } from '../board/TaskCard'
import { plural } from '../projects/filters'

const lower = (s: string) => s.toLowerCase()

// The Hangar tab: every payload in the Hangar column as one list, in board
// order. A row opens the payload drawer over the list.
export function HangarPage() {
  const project = useProjectContext()
  const { data: board, isPending, error } = useBoard(project.id)

  return (
    <>
      <div className="flex min-w-0 flex-1 flex-col gap-6 px-8 py-7">
        <header>
          <Eyebrow>
            {labels.project} · {project.name}
          </Eyebrow>
          <h1 className="mt-2 mb-0 font-heading text-[26px] font-semibold">{labels.backlog}</h1>
        </header>
        {isPending ? (
          <StatusMessage title={`Loading ${lower(labels.backlog)}…`} />
        ) : error ? (
          <StatusMessage title="Something went wrong" body={error.message} />
        ) : (
          <HangarList board={board} />
        )}
      </div>
      <Outlet />
    </>
  )
}

function HangarList({ board }: { board: Board }) {
  const tasks = board.columns.find((c) => c.key === 'backlog')?.tasks ?? []
  const epic = new Map(board.epics.map((e) => [e.id, e]))
  const feature = new Map(board.features.map((f) => [f.id, f]))
  const user = new Map(board.users.map((u) => [u.id, u]))
  const agent = new Map(board.agents.map((a) => [a.id, a]))

  if (tasks.length === 0) {
    return (
      <p role="status" className="m-0 py-10 text-center text-text-muted-strong">
        The {labels.backlog} is empty. New {lower(labels.tasks)} wait here until they launch.
      </p>
    )
  }

  /** The active launch window by number; any other is still ahead. */
  const windowOf = (t: Task) =>
    !t.sprintId ? labels.notScheduled : t.sprintId === board.sprint?.id ? labels.sprintName(board.sprint.number) : 'Upcoming'

  const owner = (t: Task) => {
    if (!t.assignee) return <span className="text-text-muted">{labels.unassigned}</span>
    if (t.assignee.kind === 'agent') {
      const a = agent.get(t.assignee.id)
      return a ? <AgentTag agent={a} active={t.status === 'working'} hail={t.status === 'hail'} /> : null
    }
    const u = user.get(t.assignee.id)
    return u ? (
      <span className="flex items-center gap-2">
        <UserAvatar user={u} size="sm" />
        {u.name}
      </span>
    ) : null
  }

  const th = 'px-4 py-3 text-left text-xs font-medium text-text-muted'
  const td = 'px-4 py-3 align-middle'

  return (
    <div className="flex flex-col gap-3">
      <p className="m-0 text-[13px] text-text-muted-strong">
        {plural(tasks.length, lower(labels.task), lower(labels.tasks))}
      </p>
      <div className="overflow-x-auto rounded-card border border-line bg-surface">
        <table aria-label={`${labels.backlog} ${lower(labels.tasks)}`} className="w-full min-w-[960px] border-collapse text-sm">
          <thead className="border-b border-line-soft bg-surface-sunken">
            <tr>
              <th scope="col" className={th}>{labels.task}</th>
              <th scope="col" className={th}>{labels.feature}</th>
              <th scope="col" className={th}>{labels.epic}</th>
              <th scope="col" className={th}>Assignee</th>
              <th scope="col" className={th}>Priority</th>
              <th scope="col" className={`${th} text-right`}>Estimate</th>
              <th scope="col" className={th}>{labels.sprint}</th>
              <th scope="col" className={th}>Status</th>
            </tr>
          </thead>
          <tbody>
            {tasks.map((t) => {
              const l = cardLook(t.status, t.assignee?.kind === 'agent')
              const e = epic.get(t.epicId)
              return (
                <tr key={t.id} className="border-b border-line-soft last:border-b-0 hover:bg-surface-sunken/60">
                  <td className={td}>
                    <Link
                      to={`/projects/${board.project.id}/backlog/tasks/${t.id}`}
                      className="flex flex-col gap-1 text-text no-underline hover:text-ion"
                    >
                      <span className="font-mono text-xs text-text-muted">{t.key}</span>
                      <span className="font-medium">{t.title}</span>
                    </Link>
                  </td>
                  <td className={`${td} text-text-muted-strong`}>{feature.get(t.featureId)?.name}</td>
                  <td className={td}>
                    {e && (
                      <span className="flex items-center gap-2 text-text-muted-strong">
                        <span aria-hidden="true" className="size-2 flex-none rounded-control" style={{ background: e.color }} />
                        {e.name}
                      </span>
                    )}
                  </td>
                  <td className={td}>{owner(t)}</td>
                  <td className={`${td} text-text-muted-strong`}>{priorityLabels[t.priority]}</td>
                  <td className={`${td} text-right font-mono text-xs text-text-muted-strong`}>{t.estimate} pt</td>
                  <td className={`${td} text-text-muted-strong`}>{windowOf(t)}</td>
                  <td className={td}>
                    <Chip tone={l.tone}>
                      {l.icon && <Icon name={l.icon} size={12} />}
                      {taskStatusLabels[t.status]}
                    </Chip>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
