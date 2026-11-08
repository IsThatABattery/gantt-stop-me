import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { Dialog } from '../../components/Dialog'
import { Eyebrow } from '../../components/Eyebrow'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { Button, Field, Textarea, TextInput } from '../../components/controls'
import { buttonClass } from '../../components/buttonClass'
import type { TaskDetail } from '../../data/api/types'
import { errorCode, NotFoundError } from '../../data/errors'
import { titleProblem, useCreateTask, useEditTask, useTask, type TaskInput } from '../../data/views/tasks'
import { actionLabels, formLabels, labels } from '../../labels/labels'
import { FieldError } from './ItemForm'

// Create and edit a payload. A payload is a GitHub issue, so the API keeps
// only its title and body; the mockups' other fields wait on the API
// (docs/implementation/api-gaps.md).

type Problems = Partial<Record<keyof TaskInput, string>>

/** Client-side checks, then the API's own refusals, on the field they belong to. */
function problemsFor(value: TaskInput, error: Error | null, showLocal: boolean): Problems {
  const out: Problems = {}
  const local = showLocal ? titleProblem(value.title) : null
  if (local) out.title = local
  const code = errorCode(error)
  if (code === 'invalid_title') out.title = error!.message
  if (code === 'invalid_body') out.body = error!.message
  return out
}

interface FieldsProps {
  value: TaskInput
  onChange: (next: TaskInput) => void
  problems: Problems
  idPrefix: string
  bodyRows: number
}

function TaskFields({ value, onChange, problems, idPrefix, bodyRows }: FieldsProps) {
  const t = formLabels.task
  const id = (name: string) => `${idPrefix}-${name}`
  return (
    <>
      <Field id={id('title')} label={t.nameLabel}>
        <TextInput
          id={id('title')}
          value={value.title}
          placeholder={t.namePlaceholder}
          onChange={(e) => onChange({ ...value, title: e.target.value })}
          aria-invalid={!!problems.title}
          aria-describedby={problems.title ? id('title-error') : undefined}
        />
        <FieldError id={id('title-error')}>{problems.title}</FieldError>
      </Field>
      <Field id={id('body')} label={formLabels.descriptionOptional}>
        <Textarea
          id={id('body')}
          rows={bodyRows}
          value={value.body}
          placeholder={t.descPlaceholder}
          onChange={(e) => onChange({ ...value, body: e.target.value })}
          aria-invalid={!!problems.body}
          aria-describedby={problems.body ? id('body-error') : undefined}
        />
        <FieldError id={id('body-error')}>{problems.body}</FieldError>
      </Field>
    </>
  )
}

/** Errors with no field of their own, such as no permission or GitHub being down. */
function GeneralError({ error }: { error: Error | null }) {
  const code = errorCode(error)
  if (!error || code === 'invalid_title' || code === 'invalid_body') return null
  return (
    <p role="alert" className="m-0 text-[13px] text-critical">
      {error.message}
    </p>
  )
}

// ---- New ----

/** Create dialog over the board (NewPayload mockup). The issue opens in Backlog. */
export function NewTaskDialog() {
  const project = useProjectContext()
  const navigate = useNavigate()
  const projectId = String(project.id)
  const create = useCreateTask(projectId)
  const [value, setValue] = useState<TaskInput>({ title: '', body: '' })
  const [submitted, setSubmitted] = useState(false)
  const close = () => navigate(`/projects/${projectId}`)
  const problems = problemsFor(value, create.error, submitted)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (titleProblem(value.title)) return
    const task = await create.mutateAsync({ title: value.title.trim(), body: value.body }).catch(() => null)
    if (task) navigate(`/projects/${projectId}/tasks/${task.id}`)
  }

  return (
    <Dialog label={formLabels.task.titleNew} onClose={close}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-[18px] p-7">
        <div className="pr-10">
          <Eyebrow>
            {`${project.owner}/${project.name}`.toUpperCase()} · {formLabels.task.eyebrowNew.toUpperCase()}
          </Eyebrow>
          <h2 className="mt-1.5 mb-0 font-heading text-2xl font-semibold">{formLabels.task.titleNew}</h2>
          <p className="mt-2 mb-0 leading-normal text-text-muted-strong">
            A {labels.task.toLowerCase()} is a GitHub issue. It opens in the {labels.backlog}.
          </p>
        </div>

        <TaskFields value={value} onChange={setValue} problems={problems} idPrefix="new-task" bodyRows={4} />
        <GeneralError error={create.error} />

        <div className="mt-2 flex flex-wrap justify-end gap-2.5 border-t border-line-soft pt-4">
          <Button onClick={close}>{actionLabels.cancel}</Button>
          <Button type="submit" variant="primary" disabled={create.isPending} className="px-[22px]">
            {create.isPending ? 'Creating…' : actionLabels.createTask}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}

// ---- Edit ----

/** Full edit page for a payload (PayloadEditor mockup, title and body only). */
export function TaskEditPage() {
  const project = useProjectContext()
  const { taskId = '' } = useParams()
  const { data: task, error } = useTask(String(project.id), taskId)

  if (error) {
    return (
      <StatusMessage
        title={error instanceof NotFoundError ? `${labels.task} not found` : 'Something went wrong'}
        body={error instanceof NotFoundError ? `Check the link, or open the ${labels.task.toLowerCase()} from the board.` : error.message}
      >
        <Link to={`/projects/${project.id}`} className={buttonClass('secondary')}>
          {actionLabels.backToBoard}
        </Link>
      </StatusMessage>
    )
  }
  if (!task) return <StatusMessage title="Loading…" />
  return <TaskEditor key={task.id} task={task} />
}

function TaskEditor({ task }: { task: TaskDetail }) {
  const project = useProjectContext()
  const navigate = useNavigate()
  const projectId = String(project.id)
  const save = useEditTask(projectId, String(task.id))
  const [saved, setSaved] = useState<TaskInput>({ title: task.title, body: task.body })
  const [value, setValue] = useState(saved)
  const [submitted, setSubmitted] = useState(false)
  const [done, setDone] = useState(false)
  const problems = problemsFor(value, save.error, submitted)
  const dirty = value.title !== saved.title || value.body !== saved.body

  const edit = (next: TaskInput) => {
    setValue(next)
    setDone(false)
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (titleProblem(value.title) || !dirty) return
    const changes: Partial<TaskInput> = {}
    if (value.title !== saved.title) changes.title = value.title.trim()
    if (value.body !== saved.body) changes.body = value.body
    const ok = await save.mutateAsync(changes).then(
      () => true,
      () => false,
    )
    if (ok) {
      const next = { ...value, title: value.title.trim() }
      setSaved(next)
      setValue(next)
      setDone(true)
    }
  }

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-6 px-8 pt-7 pb-12">
      <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-2 text-[13px] text-text-muted">
        <Link to={`/projects/${projectId}`}>{project.name}</Link>
        <span aria-hidden="true">/</span>
        <Link to={`/projects/${projectId}/tasks/${task.id}`}>#{task.issueNumber}</Link>
        <span aria-hidden="true">/</span>
        <span aria-current="page">Edit</span>
      </nav>

      <header className="flex flex-wrap items-end gap-4">
        <div className="mr-auto">
          <Eyebrow>
            {formLabels.task.eyebrowEdit.toUpperCase()} · #{task.issueNumber}
          </Eyebrow>
          <h1 className="mt-1.5 mb-0 font-heading text-[28px] font-semibold">{formLabels.task.titleEdit}</h1>
        </div>
        <div className="flex flex-wrap items-center gap-2.5">
          <span role="status" className="text-[13px]">
            {done && <span className="text-good">Saved</span>}
            {!done && dirty && <span className="text-text-muted">Unsaved changes</span>}
          </span>
          <a href={task.url} target="_blank" rel="noreferrer" className={buttonClass('link', 'px-3')}>
            <Icon name="github" size={16} />
            View on GitHub
          </a>
          <Button onClick={() => navigate(`/projects/${projectId}/tasks/${task.id}`)}>{actionLabels.cancel}</Button>
          <Button type="submit" variant="primary" disabled={save.isPending || !dirty}>
            {save.isPending ? 'Saving…' : actionLabels.save}
          </Button>
        </div>
      </header>

      <GeneralError error={save.error} />

      <section aria-labelledby="basics-heading" className="flex max-w-[820px] flex-col gap-[18px] rounded-[14px] border border-line bg-surface p-6">
        <h2 id="basics-heading" className="m-0 font-heading text-lg font-semibold">
          Basics
        </h2>
        <p className="m-0 text-[13px] text-text-muted-strong">Saving writes the title and description to the GitHub issue.</p>
        <TaskFields value={value} onChange={edit} problems={problems} idPrefix="edit-task" bodyRows={14} />
      </section>
    </form>
  )
}
