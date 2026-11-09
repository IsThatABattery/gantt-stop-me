import { useId, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { Dialog } from '../../components/Dialog'
import { Icon } from '../../components/Icon'
import { Button, Field, Textarea, TextInput } from '../../components/controls'
import { useMockSession } from '../../data/core'
import { useCreateProject } from '../../data/views/projects'
import { labels } from '../../labels/labels'

/** New project: a name and an optional description, then its empty board. */
export function NewProjectDialog({ onClose }: { onClose: () => void }) {
  const id = useId()
  const navigate = useNavigate()
  const session = useMockSession()
  const create = useCreateProject()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      setError('Give the project a name.')
      return
    }
    setError(null)
    const project = await create.mutateAsync({ name, description, userId: session?.userId ?? '' }).catch(() => null)
    if (project) navigate(`/projects/${project.id}`)
  }

  return (
    <Dialog label={labels.newProject} title={labels.newProject} onClose={onClose}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-[18px] px-6 pt-5 pb-6">
        <p className="m-0 leading-normal text-ink-2">
          A project holds one team's board, features and agents. You can add people and a sprint once it exists.
        </p>

        <Field id={`${id}-name`} label="Name">
          <TextInput
            id={`${id}-name`}
            value={name}
            placeholder="e.g. tidepool"
            onChange={(e) => setName(e.target.value)}
            aria-invalid={!!error}
            aria-describedby={error ? `${id}-error` : undefined}
          />
          {error && (
            <span id={`${id}-error`} className="flex items-start gap-1.5 text-[13px] leading-snug font-medium text-signal-ink">
              <Icon name="alert" size={14} className="mt-px flex-none" />
              {error}
            </span>
          )}
        </Field>

        <Field
          id={`${id}-desc`}
          label={
            <>
              Description <span className="text-ink-2">(optional)</span>
            </>
          }
        >
          <Textarea
            id={`${id}-desc`}
            rows={3}
            value={description}
            placeholder="What is this project for?"
            onChange={(e) => setDescription(e.target.value)}
          />
        </Field>

        {create.error && (
          <p role="alert" className="m-0 flex items-start gap-2 rounded-[2px] bg-signal-wash px-3 py-2.5 text-[13px] leading-snug font-medium text-signal-ink">
            <Icon name="alert" size={16} className="mt-px flex-none" />
            {create.error.message}
          </p>
        )}

        <div className="mt-2 flex flex-wrap justify-end gap-2.5 border-t border-keyline pt-4">
          <Button onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" busy={create.isPending} className="px-[22px]">
            {create.isPending ? 'Creating…' : 'Create project'}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
