import { useId, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { Dialog } from '../../components/Dialog'
import { Button, Field, Textarea, TextInput } from '../../components/controls'
import { useSession } from '../../data/session'
import { useCreateProject } from '../../data/views/projects'
import { labels } from '../../labels/labels'

/** New mission: a name and an optional description, then its empty board. */
export function NewProjectDialog({ onClose }: { onClose: () => void }) {
  const id = useId()
  const navigate = useNavigate()
  const session = useSession()
  const create = useCreateProject()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      setError('Give the mission a name.')
      return
    }
    setError(null)
    const project = await create.mutateAsync({ name, description, userId: session?.userId ?? '' }).catch(() => null)
    if (project) navigate(`/projects/${project.id}`)
  }

  return (
    <Dialog label={labels.newProject} onClose={onClose}>
      <form onSubmit={submit} noValidate className="flex flex-col gap-[18px] p-7">
        <div className="pr-10">
          <div className="eyebrow">{labels.flightDeck.toUpperCase()}</div>
          <h2 className="mt-1.5 mb-0 font-heading text-2xl font-semibold">{labels.newProject}</h2>
          <p className="mt-2 mb-0 leading-normal text-text-muted-strong">
            A mission holds one team's board, objectives and astronauts. You can add operators and a launch window once it exists.
          </p>
        </div>

        <Field id={`${id}-name`} label="Name">
          <TextInput
            id={`${id}-name`}
            value={name}
            placeholder="e.g. Orbital-Nav"
            onChange={(e) => setName(e.target.value)}
            aria-invalid={!!error}
            aria-describedby={error ? `${id}-error` : undefined}
          />
          {error && (
            <span id={`${id}-error`} className="text-[13px] text-critical">
              {error}
            </span>
          )}
        </Field>

        <Field
          id={`${id}-desc`}
          label={
            <>
              Description <span className="text-text-muted">(optional)</span>
            </>
          }
        >
          <Textarea
            id={`${id}-desc`}
            rows={3}
            value={description}
            placeholder="What is this mission for?"
            onChange={(e) => setDescription(e.target.value)}
          />
        </Field>

        {create.error && (
          <p role="alert" className="m-0 text-[13px] text-critical">
            {create.error.message}
          </p>
        )}

        <div className="mt-2 flex flex-wrap justify-end gap-2.5 border-t border-line-soft pt-4">
          <Button onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" disabled={create.isPending} className="px-[22px]">
            Create mission
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
