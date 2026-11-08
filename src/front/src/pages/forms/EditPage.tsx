import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { Chip, type Tone } from '../../components/Chip'
import { Eyebrow } from '../../components/Eyebrow'
import { ProgressBar } from '../../components/ProgressBar'
import { RadioCard } from '../../components/RadioCard'
import { StatusMessage } from '../../components/StatusMessage'
import { Button } from '../../components/controls'
import { buttonClass } from '../../components/buttonClass'
import { NotFoundError } from '../../data/core'
import {
  useArchiveFeature,
  useFeature,
  useFeatureTasks,
  useFormOptions,
  useTaskForEdit,
  useUpdateFeature,
  useUpdateTask,
  type FormOptions,
  type TaskForEdit,
} from '../../data/views/forms'
import type { Assignee, Autonomy, Criterion, Feature, Project, Task } from '../../data/types'
import {
  actionLabels,
  autonomyDescriptions,
  autonomyLabels,
  columnLabels,
  featureStatusLabels,
  formLabels,
  labels,
  taskStatusLabels,
} from '../../labels/labels'
import { CriteriaList } from './CriteriaList'
import { basicsErrors, featureBasics, taskBasics, toInput, type BasicsValue, type Kind } from './formValue'
import { BasicsFields } from './ItemForm'

// Full edit page for an objective or a payload (ObjectiveEditor and
// PayloadEditor mockups). The form keeps a local copy and Save sends it all.

export function FeatureEditPage() {
  const project = useProjectContext()
  const { featureId = '' } = useParams()
  const feature = useFeature(project.id, featureId)
  const options = useFormOptions(project.id)
  const error = feature.error ?? options.error

  if (error) return <LoadError error={error} kind="feature" />
  if (!feature.data || !options.data) return <StatusMessage title="Loading…" />
  return <FeatureEditor key={feature.data.id} project={project} feature={feature.data} options={options.data} />
}

export function TaskEditPage() {
  const project = useProjectContext()
  const { taskId = '' } = useParams()
  const detail = useTaskForEdit(project.id, taskId)
  const options = useFormOptions(project.id)
  const error = detail.error ?? options.error

  if (error) return <LoadError error={error} kind="task" />
  if (!detail.data || !options.data) return <StatusMessage title="Loading…" />
  return <TaskEditor key={detail.data.task.id} project={project} detail={detail.data} options={options.data} />
}

function LoadError({ error, kind }: { error: Error; kind: Kind }) {
  const { projectId } = useParams()
  const name = kind === 'feature' ? labels.feature : labels.task
  return (
    <StatusMessage
      title={error instanceof NotFoundError ? `${name} not found` : 'Something went wrong'}
      body={error instanceof NotFoundError ? `Check the link, or open the ${name.toLowerCase()} from the board.` : error.message}
    >
      <Link to={`/projects/${projectId}`} className={buttonClass('secondary')}>
        {actionLabels.backToBoard}
      </Link>
    </StatusMessage>
  )
}

const DISABLED = 'disabled:cursor-not-allowed disabled:opacity-50'

// ---- Objective ----

function FeatureEditor({ project, feature, options }: { project: Project; feature: Feature; options: FormOptions }) {
  const tasks = useFeatureTasks(project.id, feature.id)
  const save = useUpdateFeature(project.id, feature.id)
  const archive = useArchiveFeature(project.id, feature.id)
  const epic = options.epics.find((e) => e.id === feature.epicId)
  const inherited = project.defaultAutonomy

  return (
    <Editor
      kind="feature"
      project={project}
      options={options}
      initial={{ basics: featureBasics(feature), criteria: feature.successCriteria, autonomy: feature.defaultAutonomy }}
      crumbs={[
        { label: labels.features, to: `/projects/${project.id}` },
        { label: feature.name },
      ]}
      eyebrow={[formLabels.feature.eyebrowEdit, feature.key, `${labels.epic}: ${epic?.name ?? '—'}`]}
      status={feature.status === 'archived' ? featureStatusLabels.archived : null}
      inherited={{ label: labels.inheritProject, level: inherited }}
      onSave={(edit) => save.mutateAsync(edit)}
      saving={save.isPending}
      saveError={save.error}
      onArchive={feature.status === 'archived' ? undefined : () => archive.mutate()}
      side={<FeatureTasks projectId={project.id} tasks={tasks.data} options={options} />}
    />
  )
}

function FeatureTasks({ projectId, tasks, options }: { projectId: string; tasks?: Task[]; options: FormOptions }) {
  const t = formLabels.feature
  if (!tasks) return <SideCard heading={t.sideHeading}>Loading…</SideCard>
  const landed = tasks.filter((x) => x.status === 'landed').length
  const awaiting = tasks.filter((x) => x.status === 'awaiting-approval').length
  const hangar = tasks.filter((x) => x.column === 'backlog').length

  return (
    <SideCard
      heading={t.sideHeading}
      total={tasks.length}
      progress={{ label: t.progressLabel, value: landed, max: tasks.length }}
      chips={
        <>
          {awaiting > 0 && (
            <Chip tone="warning" shape="tag">
              {awaiting} {labels.awaitingApproval.toLowerCase()}
            </Chip>
          )}
          {hangar > 0 && (
            <Chip tone="neutral" shape="tag">
              {hangar} in {labels.backlog}
            </Chip>
          )}
        </>
      }
      actions={
        <>
          <Button disabled title="Needs real data" className={DISABLED}>
            {actionLabels.addExisting}
          </Button>
          <Link to={`/projects/${projectId}/new/task`} className={buttonClass('accent')}>
            {labels.newTask}
          </Link>
        </>
      }
    >
      {tasks.length === 0 ? (
        <p className="m-0 text-[13px] text-text-muted-strong">No {labels.tasks.toLowerCase()} yet.</p>
      ) : (
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {tasks.map((task) => {
            const state = taskState(task)
            return (
              <li key={task.id}>
                <Link
                  to={`/projects/${projectId}/tasks/${task.id}/edit`}
                  className="flex flex-col gap-1 rounded-control border border-line-soft bg-surface p-3 text-text no-underline hover:border-ion hover:text-text"
                >
                  <span className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs text-text-muted">{task.key}</span>
                    <Chip tone={state.tone}>{state.label}</Chip>
                  </span>
                  <span className="font-medium">{task.title}</span>
                  <span className="text-xs text-text-muted">{ownerName(task.assignee, options)}</span>
                </Link>
              </li>
            )
          })}
        </ul>
      )}
    </SideCard>
  )
}

function taskState(t: Task): { label: string; tone: Tone } {
  if (t.status === 'landed') return { label: taskStatusLabels.landed, tone: 'good' }
  if (t.status === 'blocked') return { label: taskStatusLabels.blocked, tone: 'critical' }
  if (t.status === 'awaiting-approval' || t.status === 'in-review' || t.status === 'hail') return { label: taskStatusLabels[t.status], tone: 'warning' }
  if (t.column === 'backlog') return { label: columnLabels.backlog.name, tone: 'neutral' }
  return { label: taskStatusLabels[t.status], tone: t.assignee?.kind === 'agent' ? 'astronaut' : 'ion' }
}

function ownerName(a: Assignee | null, options: FormOptions) {
  if (!a) return labels.unassigned
  if (a.kind === 'agent') return options.agents.find((x) => x.id === a.id)?.name ?? labels.agent
  return options.users.find((x) => x.id === a.id)?.initials ?? labels.user
}

// ---- Payload ----

function TaskEditor({ project, detail, options }: { project: Project; detail: TaskForEdit; options: FormOptions }) {
  const { task, subtasks } = detail
  const save = useUpdateTask(project.id, task.id)
  const feature = options.features.find((f) => f.id === task.featureId)
  const inherited = feature?.defaultAutonomy ?? project.defaultAutonomy
  const t = formLabels.task
  const done = subtasks.filter((s) => s.done).length

  return (
    <Editor
      kind="task"
      project={project}
      options={options}
      initial={{ basics: taskBasics(task), criteria: task.acceptanceCriteria, autonomy: task.autonomyOverride }}
      crumbs={[
        feature ? { label: feature.name, to: `/projects/${project.id}/features/${feature.id}/edit` } : { label: labels.tasks },
        { label: task.key },
      ]}
      eyebrow={[t.eyebrowEdit, task.key, `${labels.feature}: ${feature?.name ?? '—'}`]}
      inherited={{ label: labels.inheritFeature, level: inherited }}
      onSave={(edit) => save.mutateAsync(edit)}
      saving={save.isPending}
      saveError={save.error}
      side={
        <SideCard
          heading={t.sideHeading}
          total={subtasks.length}
          progress={{ label: t.progressLabel, value: done, max: subtasks.length }}
          chips={
            <>
              <Chip tone="neutral" shape="tag">
                {columnLabels[task.column].name} · {taskStatusLabels[task.status]}
              </Chip>
              {task.prNumber && (
                <Chip tone="neutral" shape="tag">
                  PR #{task.prNumber}
                </Chip>
              )}
            </>
          }
          actions={
            <>
              <Button disabled title="Needs real GitHub data" className={DISABLED}>
                {actionLabels.linkPr}
              </Button>
              <Button disabled title="Not built yet" className={DISABLED}>
                {actionLabels.addSubtask}
              </Button>
            </>
          }
        >
          {subtasks.length === 0 ? (
            <p className="m-0 text-[13px] text-text-muted-strong">No {labels.subtask.toLowerCase()}s yet.</p>
          ) : (
            <ul className="m-0 flex list-none flex-col gap-2 p-0">
              {subtasks.map((s, i) => (
                <li key={s.id} className="flex flex-col gap-1 rounded-control border border-line-soft bg-surface p-3">
                  <span className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs text-text-muted">M-{i + 1}</span>
                    <Chip tone={s.done ? 'good' : 'neutral'}>{s.done ? 'Done' : 'Open'}</Chip>
                  </span>
                  <span className="font-medium">{s.title}</span>
                  <span className="text-xs text-text-muted">{ownerName(s.assignee, options)}</span>
                </li>
              ))}
            </ul>
          )}
        </SideCard>
      }
    />
  )
}

// ---- Shared layout ----

interface EditValue {
  basics: BasicsValue
  criteria: Criterion[]
  autonomy: Autonomy | null
}

interface EditorProps {
  kind: Kind
  project: Project
  options: FormOptions
  initial: EditValue
  crumbs: { label: string; to?: string }[]
  eyebrow: string[]
  /** A status word shown beside the heading, e.g. Archived */
  status?: string | null
  inherited: { label: string; level: Autonomy }
  onSave: (edit: ReturnType<typeof toInput> & { criteria: Criterion[]; autonomy: Autonomy | null }) => Promise<unknown>
  saving: boolean
  saveError: Error | null
  onArchive?: () => void
  side: ReactNode
}

function Editor({ kind, project, options, initial, crumbs, eyebrow, status, inherited, onSave, saving, saveError, onArchive, side }: EditorProps) {
  const navigate = useNavigate()
  const t = formLabels[kind]
  const [value, setValue] = useState(initial)
  const [showErrors, setShowErrors] = useState(false)
  const [saved, setSaved] = useState(false)
  const errors = showErrors ? basicsErrors(kind, value.basics) : {}
  const edit = (patch: Partial<EditValue>) => {
    setSaved(false)
    setValue((v) => ({ ...v, ...patch }))
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setShowErrors(true)
    if (Object.keys(basicsErrors(kind, value.basics)).length > 0) return
    const ok = await onSave({ ...toInput(value.basics), criteria: value.criteria, autonomy: value.autonomy }).then(
      () => true,
      () => false,
    )
    setSaved(ok)
  }

  const autonomyChoices: { level: Autonomy | null; label: string; description: string }[] = [
    { level: null, label: inherited.label, description: `Currently ${autonomyLabels[inherited.level]}.` },
    ...([0, 1, 2] as const).map((level) => ({ level, label: autonomyLabels[level], description: autonomyDescriptions[level] })),
  ]

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-6 px-8 pt-7 pb-12">
      <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-2 text-[13px] text-text-muted">
        <Link to={`/projects/${project.id}`}>{project.name}</Link>
        {crumbs.map((c) => (
          <span key={c.label} className="contents">
            <span aria-hidden="true">/</span>
            {c.to ? <Link to={c.to}>{c.label}</Link> : <span aria-current="page">{c.label}</span>}
          </span>
        ))}
      </nav>

      <header className="flex flex-wrap items-end gap-4">
        <div className="mr-auto">
          <Eyebrow>{eyebrow.join(' · ').toUpperCase()}</Eyebrow>
          <h1 className="mt-2 mb-0 flex items-center gap-3 font-heading text-[28px] font-semibold">
            {t.titleEdit}
            {status && <Chip tone="neutral">{status}</Chip>}
          </h1>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <span role="status" className="text-[13px] text-good">
            {saved ? 'Saved' : ''}
          </span>
          {kind === 'feature' && (
            <Button onClick={onArchive} disabled={!onArchive} className={DISABLED}>
              {actionLabels.archive}
            </Button>
          )}
          <Button onClick={() => navigate(`/projects/${project.id}`)}>{actionLabels.cancel}</Button>
          <Button type="submit" variant="primary" disabled={saving}>
            {actionLabels.save}
          </Button>
        </div>
      </header>

      {saveError && (
        <p role="alert" className="m-0 text-[13px] text-critical">
          {saveError.message}
        </p>
      )}

      <div className="flex flex-wrap items-start gap-6">
        <div className="flex min-w-0 flex-[999_1_560px] flex-col gap-6">
          <section aria-labelledby="basics-heading" className="flex flex-col gap-5 rounded-card border border-line bg-surface p-6">
            <h2 id="basics-heading" className="m-0 font-heading text-lg font-semibold">
              Basics
            </h2>
            <BasicsFields
              kind={kind}
              value={value.basics}
              onChange={(basics) => edit({ basics })}
              options={options}
              errors={errors}
              variant="page"
              idPrefix={`edit-${kind}`}
            />
          </section>

          <CriteriaList heading={t.criteriaHeading} help={t.criteriaHelp} criteria={value.criteria} onChange={(criteria) => edit({ criteria })} />

          <section aria-labelledby="autonomy-heading" className="flex flex-col gap-4 rounded-card border border-line bg-surface p-6">
            <h2 id="autonomy-heading" className="m-0 font-heading text-lg font-semibold">
              {labels.agents}
            </h2>
            <fieldset className="m-0 grid gap-3 border-0 p-0 sm:grid-cols-2">
              <legend className="mb-3 p-0 text-[13px] text-text-soft">{t.autonomyLegend}</legend>
              {autonomyChoices.map((c) => (
                <RadioCard
                  key={c.level ?? 'inherit'}
                  name={`${kind}-autonomy`}
                  tone="astronaut"
                  checked={value.autonomy === c.level}
                  onChange={() => edit({ autonomy: c.level })}
                  label={c.label}
                  description={c.description}
                />
              ))}
            </fieldset>
          </section>
        </div>

        {side}
      </div>
    </form>
  )
}

interface SideCardProps {
  heading: string
  total?: number
  progress?: { label: string; value: number; max: number }
  chips?: ReactNode
  actions?: ReactNode
  children: ReactNode
}

function SideCard({ heading, total, progress, chips, actions, children }: SideCardProps) {
  return (
    <aside aria-label={heading} className="flex min-w-0 flex-[1_1_340px] flex-col gap-4 rounded-card border border-line bg-surface-sunken p-6">
      <div className="flex items-baseline justify-between">
        <h2 className="m-0 font-heading text-lg font-semibold">{heading}</h2>
        {total !== undefined && <span className="text-[13px] text-text-muted">{total} total</span>}
      </div>
      {progress && <ProgressBar {...progress} />}
      {chips && <div className="flex flex-wrap gap-2">{chips}</div>}
      {children}
      {actions && <div className="flex flex-wrap gap-3">{actions}</div>}
    </aside>
  )
}
