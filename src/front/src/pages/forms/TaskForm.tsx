import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { useProjectContext } from '../../app/projectContext'
import { Dialog } from '../../components/Dialog'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { Button, Field, Textarea, TextInput } from '../../components/controls'
import { buttonClass } from '../../components/buttonClass'
import { PageTitle } from '../../components/PageTitle'
import { InkTab, Tile } from '../../components/Tile'
import { UnsavedChangesGuard } from '../../components/UnsavedChangesGuard'
import type { TaskDetail } from '../../data/api/types'
import { errorCode, NotFoundError } from '../../data/errors'
import { titleProblem, useCreateTask, useEditTask, useTask, type TaskInput } from '../../data/views/tasks'
import { actionLabels, formLabels, labels } from '../../labels/labels'
import { FieldError } from './ItemForm'

// Create and edit a task. A task is a GitHub issue, so the API keeps only its
// title and body; the mockups' other fields wait on the API
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
          aria-describedby={[id('body-help'), problems.body && id('body-error')].filter(Boolean).join(' ')}
          className="resize-y"
        />
        <FieldError id={id('body-error')}>{problems.body}</FieldError>
        <p id={id('body-help')} className="m-0 text-xs leading-normal text-ink-2">
          Markdown, as on GitHub. Tests go under a <code className="font-mono text-ink">## Tests</code> heading.
        </p>
      </Field>
    </>
  )
}

/** Errors with no field of their own, such as no permission or GitHub being down. */
function GeneralError({ error }: { error: Error | null }) {
  const code = errorCode(error)
  if (!error || code === 'invalid_title' || code === 'invalid_body') return null
  return (
    <p role="alert" className="m-0 flex items-start gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] leading-normal text-signal-ink">
      <Icon name="alert" size={16} className="mt-px flex-none" />
      {error.message}
    </p>
  )
}

// ---- New ----

/** Create dialog over the board. The task is a new GitHub issue, and it opens in the Backlog. */
export function NewTaskDialog() {
  const project = useProjectContext()
  const navigate = useNavigate()
  const projectId = String(project.id)
  const create = useCreateTask(projectId)
  const [value, setValue] = useState<TaskInput>({ title: '', body: '' })
  const [submitted, setSubmitted] = useState(false)
  const close = () => navigate(`/projects/${projectId}`)
  const problems = problemsFor(value, create.error, submitted)
  // Start in Title. This runs after the Dialog put focus on its first control, the close button.
  useEffect(() => {
    document.getElementById('new-task-title')?.focus()
  }, [])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (titleProblem(value.title)) {
      document.getElementById('new-task-title')?.focus()
      return
    }
    const task = await create.mutateAsync({ title: value.title.trim(), body: value.body }).catch(() => null)
    if (task) navigate(`/projects/${projectId}/tasks/${task.id}`)
  }

  return (
    <Dialog label={formLabels.task.titleNew} title={formLabels.task.titleNew} onClose={close}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-5 px-6 pt-5 pb-6 max-sm:px-4">
        <p className="m-0 text-sm leading-normal text-ink-2">
          A {labels.task.toLowerCase()} is an issue in{' '}
          <span className="font-semibold text-ink">
            {project.owner}/{project.name}
          </span>
          . It opens in the {labels.backlog}.
        </p>

        <TaskFields value={value} onChange={setValue} problems={problems} idPrefix="new-task" bodyRows={5} />
        <GeneralError error={create.error} />

        <div className="-mx-6 flex flex-wrap justify-end gap-2.5 border-t border-rule px-6 pt-5 max-sm:-mx-4 max-sm:px-4">
          <Button onClick={close}>{actionLabels.cancel}</Button>
          <Button type="submit" variant="primary" busy={create.isPending} className="px-[22px]">
            {create.isPending ? 'Creating…' : actionLabels.createTask}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}

// ---- Edit ----

/** Full edit page for a task: its issue's title and description. */
export function TaskEditPage() {
  const project = useProjectContext()
  const { taskId = '' } = useParams()
  const { data: task, error } = useTask(String(project.id), taskId)

  if (error) {
    return (
      <StatusMessage
        tone={error instanceof NotFoundError ? 'neutral' : 'signal'}
        icon={error instanceof NotFoundError ? 'search' : undefined}
        title={error instanceof NotFoundError ? `${labels.task} not found` : 'Something went wrong'}
        body={error instanceof NotFoundError ? `Check the link, or open the ${labels.task.toLowerCase()} from the board.` : error.message}
      >
        <Link to={`/projects/${project.id}`} className={buttonClass('secondary')}>
          {actionLabels.backToBoard}
        </Link>
      </StatusMessage>
    )
  }
  if (!task) return <StatusMessage title={`Loading ${labels.task.toLowerCase()}…`} />
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
  const drawer = `/projects/${projectId}/tasks/${task.id}`

  const edit = (next: TaskInput) => {
    setValue(next)
    setDone(false)
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitted(true)
    if (titleProblem(value.title)) {
      document.getElementById('edit-task-title')?.focus()
      return
    }
    if (!dirty) return
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
    <form onSubmit={submit} noValidate className="flex flex-col gap-6 px-4 pt-5 pb-12 md:px-7">
      <header className="flex flex-col gap-4">
        <nav aria-label="Breadcrumb">
          <ol className="m-0 flex list-none flex-wrap items-center gap-1 p-0 text-[13px] font-semibold text-ink-2">
            <li>
              <Link to={`/projects/${projectId}`} className="inline-flex min-h-8 items-center">
                {project.name}
              </Link>
            </li>
            <li aria-hidden="true" className="flex">
              <Icon name="chevronRight" size={14} />
            </li>
            <li>
              <Link to={drawer} className="inline-flex min-h-8 items-center font-mono">
                #{task.issueNumber}
              </Link>
            </li>
            <li aria-hidden="true" className="flex">
              <Icon name="chevronRight" size={14} />
            </li>
            <li aria-current="page" className="text-ink">
              Edit
            </li>
          </ol>
        </nav>

        <div className="flex flex-wrap items-end justify-between gap-x-8 gap-y-4">
          <div className="flex min-w-0 flex-col gap-2.5">
            <PageTitle>{formLabels.task.titleEdit}</PageTitle>
            <p className="m-0 text-[15px] break-words text-ink-2">
              <span className="font-mono font-semibold text-ink">#{task.issueNumber}</span> · {saved.title}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2.5">
            <span role="status" className="inline-flex items-center gap-1.5 text-[13px] font-semibold">
              {done && (
                <>
                  <Icon name="check" size={16} className="text-good" />
                  <span className="text-ink">Saved</span>
                </>
              )}
              {!done && dirty && <span className="text-ink-2">Unsaved changes</span>}
            </span>
            <a href={task.url} target="_blank" rel="noreferrer" className={buttonClass('link', 'px-2')}>
              <Icon name="github" size={16} />
              View on GitHub
            </a>
            <Button onClick={() => navigate(drawer)}>{actionLabels.cancel}</Button>
            <Button type="submit" variant="primary" busy={save.isPending} disabled={!dirty} className="px-[22px]">
              {save.isPending ? 'Saving…' : actionLabels.save}
            </Button>
          </div>
        </div>
        <UnsavedChangesGuard when={dirty && !save.isPending} />
      </header>

      <GeneralError error={save.error} />

      <div className="grid max-w-[1160px] items-start gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <Tile
          as="section"
          aria-labelledby="issue-heading"
          frameClassName="gap-4"
          tab={<InkTab id="issue-heading">Issue</InkTab>}
        >
          <p className="m-0 px-0.5 text-[13px] leading-[1.45] text-ink-2">
            Saving writes the title and description to issue #{task.issueNumber} on GitHub.
          </p>
          <TaskFields value={value} onChange={edit} problems={problems} idPrefix="edit-task" bodyRows={16} />
        </Tile>

        <Tile
          as="aside"
          step={1}
          aria-labelledby="tests-heading"
          frameClassName="gap-3"
          tab={
            <InkTab
              id="tests-heading"
              count={task.tests.length}
              countLabel="tests"
              mark={task.testsError ? 'signal' : undefined}
              markLabel={task.testsError ? 'The Tests section has a problem' : undefined}
            >
              Tests
            </InkTab>
          }
        >
          <p className="m-0 px-0.5 text-[13px] leading-[1.45] text-ink-2">
            Tests live in the description, under a <code className="font-mono text-ink">## Tests</code> heading. Each item is a command in
            backticks or a plain-English check for an LLM, with its steps indented below. A{' '}
            <code className="font-mono text-ink">[workflow]</code> in front picks where it runs.
          </p>
          <pre className="m-0 overflow-x-auto rounded-card border border-line bg-paper px-3 py-2.5 font-mono text-xs leading-relaxed whitespace-pre-wrap text-ink">
            {EXAMPLE}
          </pre>
          {task.testsError && (
            <p className="m-0 flex items-start gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] leading-normal text-signal-ink">
              <Icon name="alert" size={16} className="mt-px flex-none" />
              {task.testsError}
            </p>
          )}
          {task.tests.length > 0 && (
            <h3 className="m-0 mt-1 px-0.5 text-[13px] font-semibold text-ink-2">In this issue now</h3>
          )}
          {task.tests.length > 0 && (
            <ul aria-label="Tests in this issue" className="m-0 flex list-none flex-col gap-1.5 p-0">
              {task.tests.map((t, i) => (
                <li key={i} className="rounded-card border border-line bg-paper px-3 py-2 text-[13px] leading-snug break-words">
                  <span className={t.kind === 'script' && t.command === t.name ? 'font-mono font-semibold' : 'font-semibold'}>{t.name}</span>
                  <span className="block text-xs text-ink-2">
                    {t.kind === 'script' ? 'Command' : 'LLM check'}
                    {t.workflow && ` · ${t.workflow} workflow`}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </Tile>
      </div>
    </form>
  )
}

const EXAMPLE = `## Tests
- [ ] \`npm test\`
- [ ] [frontend] Fits on one page
  1. Open a board.
  2. Print it.`
