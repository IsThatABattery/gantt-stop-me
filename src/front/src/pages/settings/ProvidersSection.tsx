import { useEffect, useId, useState, type FormEvent } from 'react'
import { flushSync } from 'react-dom'
import { Chip } from '../../components/Chip'
import { Button, Field, FieldError, TextInput } from '../../components/controls'
import { ModelCombobox } from '../../components/ModelCombobox'
import { Icon } from '../../components/Icon'
import type { Provider } from '../../data/api/types'
import { errorCode } from '../../data/errors'
import { updateProvider, useCreateProvider, useDeleteProvider, useProviders, useUpdateProvider } from '../../data/views/settings'
import { actionLabels, settingsLabels } from '../../labels/labels'
import { Card, Confirm, Problem, Section } from './Section'
import { sectionIds } from './sections'

// The user's LLM providers (docs/api/providers.md). Adding one checks the
// endpoint and lists its models, so models are picked after it's saved.

type FieldName = 'name' | 'baseUrl' | 'apiKey' | 'models'

/** Which field each provider error belongs to; the rest show under the form. */
const fieldOf: Record<string, FieldName> = {
  invalid_name: 'name',
  name_taken: 'name',
  invalid_base_url: 'baseUrl',
  provider_unreachable: 'baseUrl',
  invalid_api_key: 'apiKey',
  too_many_models: 'models',
  duplicate_model: 'models',
  unknown_model: 'models',
}

function splitError(error: Error | null) {
  const code = errorCode(error)
  const field = code ? fieldOf[code] : undefined
  return {
    on: (name: FieldName) => (field === name ? error?.message : undefined),
    general: error && !field ? error.message : null,
  }
}

export function ProvidersSection() {
  const { data: providers, error, refetch } = useProviders()
  const [adding, setAdding] = useState(false)
  /** A provider just added, waiting for its models to be picked */
  const [picking, setPicking] = useState<{ id: number; available: string[] } | null>(null)
  const [editing, setEditing] = useState<number | null>(null)

  return (
    <Section
      id={sectionIds.providers}
      title={settingsLabels.providers}
      intro={settingsLabels.providersHelp}
      count={providers?.length}
      countLabel={providers?.length === 1 ? 'provider' : 'providers'}
    >
      {/* A failed refetch keeps the list, which may then miss a provider just added. */}
      {error && (
        <Problem alert action={<Button onClick={() => refetch()}>Try again</Button>}>
          Couldn't load providers. {error.message}
        </Problem>
      )}
      {!providers ? (
        !error && (
          <p role="status" className="m-0 px-0.5 text-[13px] text-ink-2">
            Loading providers…
          </p>
        )
      ) : (
        <>
          {providers.length === 0 && !adding && (
            <Card>
              <p className="m-0 max-w-[60ch] text-sm leading-normal text-ink-2">
                <span className="font-semibold text-ink">No providers yet.</span> Add an OpenAI-compatible endpoint, such as OpenAI itself
                or a local Ollama, to choose your agents' models.
              </p>
            </Card>
          )}
          {providers.length > 0 && (
            <ul className="m-0 flex list-none flex-col gap-2 p-0">
              {providers.map((p) => (
                <Card key={p.id} as="li">
                  {editing === p.id ? (
                    <EditProvider provider={p} onDone={() => setEditing(null)} />
                  ) : picking?.id === p.id ? (
                    <PickModels provider={p} available={picking.available} onDone={() => setPicking(null)} />
                  ) : (
                    <ProviderRow provider={p} onEdit={() => setEditing(p.id)} />
                  )}
                </Card>
              ))}
            </ul>
          )}
          {adding ? (
            <Card>
              <AddProvider
                onCancel={() => setAdding(false)}
                onAdded={(id, available) => {
                  setAdding(false)
                  setPicking({ id, available })
                }}
              />
            </Card>
          ) : (
            <div>
              <Button onClick={() => setAdding(true)}>
                <Icon name="plus" size={16} />
                Add provider
              </Button>
            </div>
          )}
        </>
      )}
    </Section>
  )
}

function ProviderRow({ provider, onEdit }: { provider: Provider; onEdit: () => void }) {
  const remove = useDeleteProvider()
  const id = useId()
  const [confirming, setConfirming] = useState(false)
  const confirm = (open: boolean) => {
    flushSync(() => setConfirming(open))
    document.getElementById(`${id}-${open ? 'keep' : 'remove'}`)?.focus()
  }
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-start gap-x-4 gap-y-3">
        <div className="flex min-w-[min(100%,14rem)] flex-1 flex-col gap-1">
          <h3 className="m-0 text-base font-bold wrap-anywhere">{provider.name}</h3>
          <span className="font-mono text-xs break-all text-ink-2">{provider.baseUrl}</span>
        </div>
        <span className="flex flex-none gap-2">
          <Button onClick={onEdit} aria-label={`Edit ${provider.name}`}>
            <Icon name="edit" size={16} />
            Edit
          </Button>
          {!confirming && (
            <Button id={`${id}-remove`} onClick={() => confirm(true)} aria-label={`Remove ${provider.name}`}>
              Remove
            </Button>
          )}
        </span>
      </div>
      {confirming && (
        <Confirm id={id} question={`Remove ${provider.name}? Its saved key is deleted too.`} onCancel={() => confirm(false)}>
          <Button id={`${id}-keep`} onClick={() => confirm(false)}>
            Keep it
          </Button>
          <Button
            variant="danger"
            // On failure, back to the row's Remove button, with the error right under it.
            onClick={() => remove.mutate(provider.id, { onError: () => confirm(false) })}
            busy={remove.isPending}
          >
            Remove
          </Button>
        </Confirm>
      )}
      {remove.error && <Problem alert>{remove.error.message}</Problem>}
      <dl className="m-0 grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-4 gap-y-2 border-t border-keyline pt-3 text-[13px]">
        <dt className="inline-flex items-center gap-1.5 font-semibold text-ink-2">
          <Icon name="lock" size={14} />
          Key
        </dt>
        <dd className="m-0 font-mono text-ink">{provider.keyLast4 ? `•••• ${provider.keyLast4}` : 'saved'}</dd>
        <dt className="font-semibold text-ink-2">Models</dt>
        <dd className="m-0 flex min-w-0 flex-wrap gap-1.5">
          {provider.models.length ? (
            provider.models.map((m) => (
              <Chip key={m} tone="neutral" shape="tag">
                {/* A chip keeps one line, so an id too long for the card ends in an ellipsis, with the whole id on hover. */}
                <span title={m} className="min-w-0 truncate">
                  {m}
                </span>
              </Chip>
            ))
          ) : (
            <span className="text-ink-2">None picked yet. Edit to pick them.</span>
          )}
        </dd>
      </dl>
    </div>
  )
}

function AddProvider({ onCancel, onAdded }: { onCancel: () => void; onAdded: (id: number, available: string[]) => void }) {
  const create = useCreateProvider()
  const [form, setForm] = useState({ name: '', baseUrl: '', apiKey: '' })
  const errors = splitError(create.error)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    create.mutate(
      { name: form.name.trim(), baseUrl: form.baseUrl.trim(), apiKey: form.apiKey },
      { onSuccess: (p) => onAdded(p.id, p.availableModels) },
    )
  }

  return (
    <form onSubmit={submit} noValidate aria-label="Add provider" className="flex flex-col gap-4">
      <h3 className="m-0 text-base font-bold">Add a provider</h3>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,220px),1fr))] gap-4">
        <TextField id="provider-name" label="Name" value={form.name} placeholder="e.g. OpenAI" error={errors.on('name')} onChange={(name) => setForm({ ...form, name })} />
        <TextField
          id="provider-url"
          label="Base URL"
          value={form.baseUrl}
          placeholder="https://api.openai.com/v1"
          error={errors.on('baseUrl')}
          onChange={(baseUrl) => setForm({ ...form, baseUrl })}
        />
      </div>
      <TextField id="provider-key" label={settingsLabels.apiKey} type="password" value={form.apiKey} error={errors.on('apiKey')} onChange={(apiKey) => setForm({ ...form, apiKey })} />
      {errors.general && <Problem alert>{errors.general}</Problem>}
      <div className="flex flex-wrap justify-end gap-2.5">
        <Button onClick={onCancel}>{actionLabels.cancel}</Button>
        <Button type="submit" variant="primary" busy={create.isPending}>
          {create.isPending ? 'Checking the endpoint…' : 'Add provider'}
        </Button>
      </div>
    </form>
  )
}

/** Right after adding: choose from the models the endpoint listed. */
function PickModels({ provider, available, onDone }: { provider: Provider; available: string[]; onDone: () => void }) {
  const update = useUpdateProvider(provider.id)
  const [models, setModels] = useState(provider.models)
  const errors = splitError(update.error)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    update.mutate({ models }, { onSuccess: onDone })
  }

  return (
    <form onSubmit={submit} noValidate aria-label={`Pick models for ${provider.name}`} className="flex flex-col gap-3">
      <p className="m-0 flex items-center gap-2">
        <Icon name="check" size={16} className="flex-none text-good" />
        <span className="min-w-0 wrap-anywhere">
          <span className="font-bold">{provider.name}</span> is added. Pick the models to use from it.
        </span>
      </p>
      <ModelPicker available={available} chosen={models} onChange={setModels} error={errors.on('models')} idPrefix={`pick-${provider.id}`} />
      {errors.general && <Problem alert>{errors.general}</Problem>}
      <div className="flex flex-wrap justify-end gap-2.5">
        <Button onClick={onDone}>Later</Button>
        <Button type="submit" variant="primary" busy={update.isPending}>
          Save models
        </Button>
      </div>
    </form>
  )
}

function EditProvider({ provider, onDone }: { provider: Provider; onDone: () => void }) {
  const update = useUpdateProvider(provider.id)
  const [available, setAvailable] = useState<string[] | null>(null)
  const [listError, setListError] = useState<Error | null>(null)
  const [form, setForm] = useState({ baseUrl: provider.baseUrl, apiKey: '', models: provider.models })
  const errors = splitError(update.error)
  const urlChanged = form.baseUrl.trim() !== provider.baseUrl

  // An empty update asks the endpoint for its models again and changes nothing.
  useEffect(() => {
    let current = true
    updateProvider(provider.id, {}).then(
      (p) => current && setAvailable(p.availableModels),
      (e: Error) => current && setListError(e),
    )
    return () => {
      current = false
    }
  }, [provider.id])

  const submit = (e: FormEvent) => {
    e.preventDefault()
    update.mutate(
      {
        ...(urlChanged ? { baseUrl: form.baseUrl.trim() } : {}),
        ...(form.apiKey ? { apiKey: form.apiKey } : {}),
        models: form.models,
      },
      { onSuccess: onDone },
    )
  }

  return (
    <form onSubmit={submit} noValidate aria-label={`Edit ${provider.name}`} className="flex flex-col gap-4">
      <h3 className="m-0 text-base font-bold wrap-anywhere">Edit {provider.name}</h3>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,220px),1fr))] gap-4">
        <TextField id={`edit-${provider.id}-url`} label="Base URL" value={form.baseUrl} error={errors.on('baseUrl')} onChange={(baseUrl) => setForm({ ...form, baseUrl })} />
        <TextField
          id={`edit-${provider.id}-key`}
          label={settingsLabels.apiKey}
          type="password"
          value={form.apiKey}
          placeholder={urlChanged ? 'Needed again for a new URL' : 'Leave blank to keep the saved key'}
          error={errors.on('apiKey')}
          onChange={(apiKey) => setForm({ ...form, apiKey })}
        />
      </div>
      {available ? (
        <ModelPicker available={available} chosen={form.models} onChange={(models) => setForm({ ...form, models })} error={errors.on('models')} idPrefix={`edit-${provider.id}`} />
      ) : listError ? (
        <Problem>Couldn't list the models. {listError.message}</Problem>
      ) : (
        <p role="status" className="m-0 text-[13px] text-ink-2">
          Listing models…
        </p>
      )}
      {errors.general && <Problem alert>{errors.general}</Problem>}
      <div className="flex flex-wrap justify-end gap-2.5">
        <Button onClick={onDone}>{actionLabels.cancel}</Button>
        <Button type="submit" variant="primary" busy={update.isPending}>
          {actionLabels.save}
        </Button>
      </div>
    </form>
  )
}

function ModelPicker({
  available,
  chosen,
  onChange,
  error,
  idPrefix,
}: {
  available: string[]
  chosen: string[]
  onChange: (models: string[]) => void
  error?: string
  idPrefix: string
}) {
  // Keep models chosen before even if the endpoint stopped listing them, so they can be unticked.
  const all = [...available, ...chosen.filter((m) => !available.includes(m))]
  return (
    <fieldset className="m-0 flex flex-col gap-1 border-0 p-0" aria-describedby={error ? `${idPrefix}-models-error` : undefined}>
      <legend className="mb-1.5 p-0 text-[13px] font-semibold text-ink-2">Models</legend>
      {all.length === 0 ? (
        <p className="m-0 text-[13px] text-ink-2">The endpoint lists no models.</p>
      ) : (
        <ModelCombobox id={`${idPrefix}-models`} aria-label="Models" multiple max={100} options={all} selected={chosen} onChange={onChange} placeholder="Search models" />
      )}
      <FieldError id={`${idPrefix}-models-error`}>{error}</FieldError>
    </fieldset>
  )
}

function TextField({
  id,
  label,
  value,
  onChange,
  error,
  type = 'text',
  placeholder,
}: {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  error?: string
  type?: 'text' | 'password'
  placeholder?: string
}) {
  return (
    <Field id={id} label={label}>
      <TextInput
        id={id}
        type={type}
        autoComplete="off"
        spellCheck={false}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={!!error}
        aria-describedby={error ? `${id}-error` : undefined}
      />
      <FieldError id={`${id}-error`}>{error}</FieldError>
    </Field>
  )
}
