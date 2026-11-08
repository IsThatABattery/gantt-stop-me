import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { Dialog } from '../../components/Dialog'
import { Eyebrow } from '../../components/Eyebrow'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import { Button } from '../../components/controls'
import { buttonClass } from '../../components/buttonClass'
import { useMockProject, useMockSession } from '../../data/core'
import { useCreateFeature, useCreateTask, useFormOptions, type FormOptions } from '../../data/views/forms'
import type { Project } from '../../data/types'
import { actionLabels, formLabels } from '../../labels/labels'
import { basicsErrors, emptyBasics, toInput, type Kind } from './formValue'
import { BasicsFields } from './ItemForm'

/** Sample draft the planner opens on until drafts can be created. */
const SAMPLE_BRIEFING = 'realtime-briefing'

/** Create dialog over the board (NewObjective / NewPayload mockups). */
export function NewItemDialog({ kind }: { kind: Kind }) {
  const { projectId = '' } = useParams()
  const navigate = useNavigate()
  const t = formLabels[kind]
  const project = useMockProject(projectId)
  const options = useFormOptions(projectId)
  const close = () => navigate(`/projects/${projectId}`)

  return (
    <Dialog label={t.titleNew} onClose={close}>
      {options.data && project.data ? (
        <NewItemForm kind={kind} project={project.data} options={options.data} onCancel={close} />
      ) : options.error ? (
        <StatusMessage title="Something went wrong" body={options.error.message} />
      ) : (
        <StatusMessage title="Loading…" />
      )}
    </Dialog>
  )
}

interface NewItemFormProps {
  kind: Kind
  project: Project
  options: FormOptions
  onCancel: () => void
}

function NewItemForm({ kind, project, options, onCancel }: NewItemFormProps) {
  const navigate = useNavigate()
  const session = useMockSession()
  const t = formLabels[kind]
  const [value, setValue] = useState(() =>
    emptyBasics(kind, options, { userId: session?.userId ?? '', sprintId: project.activeSprintId }),
  )
  const [showErrors, setShowErrors] = useState(false)
  const createFeature = useCreateFeature(project.id)
  const createTask = useCreateTask(project.id)
  const mutation = kind === 'feature' ? createFeature : createTask
  const errors = showErrors ? basicsErrors(kind, value) : {}

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setShowErrors(true)
    if (Object.keys(basicsErrors(kind, value)).length > 0) return
    const base = `/projects/${project.id}`
    if (kind === 'feature') {
      const f = await createFeature.mutateAsync(toInput(value)).catch(() => null)
      if (f) navigate(`${base}/features/${f.id}/edit`)
    } else {
      const task = await createTask.mutateAsync(toInput(value)).catch(() => null)
      if (task) navigate(`${base}/tasks/${task.id}/edit`)
    }
  }

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-[18px] p-7">
      <div className="pr-10">
        <Eyebrow>
          {project.name.toUpperCase()} · {t.eyebrowNew.toUpperCase()}
        </Eyebrow>
        <h2 className="mt-1.5 mb-0 font-heading text-2xl font-semibold">{t.titleNew}</h2>
        <p className="mt-2 mb-0 leading-normal text-text-muted-strong">{t.intro}</p>
      </div>

      <BasicsFields kind={kind} value={value} onChange={setValue} options={options} errors={errors} variant="dialog" idPrefix={`new-${kind}`} />

      {mutation.error && (
        <p role="alert" className="m-0 text-[13px] text-critical">
          {mutation.error.message}
        </p>
      )}

      <div className="mt-2 flex flex-wrap justify-end gap-2.5 border-t border-line-soft pt-4">
        {kind === 'feature' && (
          <Link
            to={`/projects/${project.id}/plan/${SAMPLE_BRIEFING}/briefing`}
            className={buttonClass('secondary', 'mr-auto border-astronaut bg-astronaut/10 hover:border-astronaut')}
          >
            <Icon name="helmet" className="text-astronaut" />
            {actionLabels.planWithAgent}
          </Link>
        )}
        <Button onClick={onCancel}>{actionLabels.cancel}</Button>
        <Button type="submit" variant="primary" disabled={mutation.isPending} className="px-[22px]">
          {kind === 'feature' ? actionLabels.createFeature : actionLabels.createTask}
        </Button>
      </div>
    </form>
  )
}
