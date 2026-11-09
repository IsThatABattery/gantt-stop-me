import { Icon } from '../../components/Icon'
import { ModelCombobox } from '../../components/ModelCombobox'
import type { TaskDetail } from '../../data/api/types'
import { useAssign, useAssignable } from '../../data/views/tasks'

/** Who the task is assigned to; a change saves to its GitHub issue at once. */
export function Assignees({ projectId, task }: { projectId: string; task: TaskDetail }) {
  const assignable = useAssignable(projectId)
  const assign = useAssign(projectId, String(task.id))
  const current = task.assignees.map((a) => a.login)
  // Someone assigned on GitHub who is no longer assignable can still be removed.
  const options = [...(assignable.data ?? []).map((a) => a.login), ...current.filter((l) => !assignable.data?.some((a) => a.login === l))]
  const error = assign.error ?? assignable.error

  return (
    <div className="flex min-w-0 flex-col gap-2">
      {task.assignees.length > 0 ? (
        <ul className="m-0 flex list-none flex-wrap gap-1.5 p-0">
          {task.assignees.map((a) => (
            <li key={a.login} className="inline-flex h-7 items-center gap-1.5 rounded-[2px] bg-block pr-2.5 pl-0.5 text-[13px] font-semibold text-ink">
              <img src={a.avatarUrl} alt="" width={24} height={24} className="size-6 rounded-[2px] bg-paper" />
              {a.login}
            </li>
          ))}
        </ul>
      ) : (
        <p className="m-0 text-[13px] leading-6 text-ink-2">No one yet.</p>
      )}
      <ModelCombobox
        id={`assignees-${task.id}`}
        aria-label="Assignees"
        multiple
        options={options}
        selected={current}
        onChange={(logins) => assign.mutate(logins)}
        placeholder={assignable.isPending ? 'Loading people…' : 'Assign people'}
        disabled={assign.isPending || !!assignable.error}
      />
      {assign.isPending && (
        <p role="status" className="m-0 text-xs text-ink-2">
          Saving to GitHub…
        </p>
      )}
      {error && (
        <p role="alert" className="m-0 flex items-start gap-1.5 text-[13px] text-signal-ink">
          <Icon name="alert" size={16} className="mt-px flex-none" />
          {error.message}
        </p>
      )}
    </div>
  )
}
