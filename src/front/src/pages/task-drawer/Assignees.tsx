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

  return (
    <section className="flex flex-col gap-2">
      <h2 className="eyebrow m-0">Assignees</h2>
      {task.assignees.length > 0 && (
        <ul className="m-0 flex list-none flex-wrap gap-2 p-0">
          {task.assignees.map((a) => (
            <li key={a.login} className="inline-flex items-center gap-1.5 text-sm text-text-soft">
              <img src={a.avatarUrl} alt="" className="size-5 rounded-full" />
              {a.login}
            </li>
          ))}
        </ul>
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
      {(assign.error ?? assignable.error) && (
        <p role="alert" className="m-0 text-[13px] text-critical">
          {(assign.error ?? assignable.error)!.message}
        </p>
      )}
    </section>
  )
}
