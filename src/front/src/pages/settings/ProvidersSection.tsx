import { useEffect, useState, type FormEvent } from 'react'
import { Button, Field, TextInput } from '../../components/controls'
import { ModelCombobox } from '../../components/ModelCombobox'
import { Icon } from '../../components/Icon'
import { StatusMessage } from '../../components/StatusMessage'
import type { Provider } from '../../data/api/types'
import { errorCode } from '../../data/errors'
import { updateProvider, useCreateProvider, useDeleteProvider, useProviders, useUpdateProvider } from '../../data/views/settings'
import { actionLabels, settingsLabels } from '../../labels/labels'
import { FieldError } from '../forms/ItemForm'
import { Section } from './Section'
import { sectionIds } from './sections'

// The user's LLM providers (docs/providers.md). Adding one checks the
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
  const { data: providers, error } = useProviders()
  const [adding, setAdding] = useState(false)
  /** A provider just added, waiting for its models to be picked */
  const [picking, setPicking] = useState<{ id: number; available: string[] } | null>(null)
  const [editing, setEditing] = useState<number | null>(null)

  return (
    <Section id={sectionIds.providers} title={settingsLabels.providers} intro={settingsLabels.providersHelp}>
      {error ? (
        <StatusMessage title="Couldn't load providers" body={error.message} />
      ) : !providers ? (
        <p className="m-0 text-[13px] text-text-muted">Loading…</p>
      ) : (
        <>
          {providers.length === 0 && !adding && <p className="m-0 text-[13px] text-text-muted-strong">No providers yet.</p>}
          <ul className="m-0 flex list-none flex-col gap-3 p-0">
            {providers.map((p) => (
              <li key={p.id} className="rounded-control border border-line-soft bg-surface-sunken p-4">
                {editing === p.id ? (
                  <EditProvider provider={p} onDone={() => setEditing(null)} />
                ) : picking?.id === p.id ? (
                  <PickModels provider={p} available={picking.available} onDone={() => setPicking(null)} />
                ) : (
                  <ProviderRow provider={p} onEdit={() => setEditing(p.id)} />
                )}
              </li>
            ))}
          </ul>
          {adding ? (
            <AddProvider
              onCancel={() => setAdding(false)}
              onAdded={(id, available) => {
                setAdding(false)
                setPicking({ id, available })
              }}
            />
          ) : (
            <div>
              <Button variant="accent" onClick={() => setAdding(true)}>
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
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-3">
        <span className="font-medium">{provider.name}</span>
        <span className="font-mono text-xs text-text-muted">{provider.baseUrl}</span>
        <span className="ml-auto flex gap-2">
          <Button onClick={onEdit} aria-label={`Edit ${provider.name}`}>
            Edit
          </Button>
          <Button onClick={() => remove.mutate(provider.id)} disabled={remove.isPending} aria-label={`Remove ${provider.name}`}>
            Remove
          </Button>
        </span>
      </div>
      <span className="inline-flex items-center gap-2 text-[13px] text-text-muted-strong">
        <Icon name="lock" size={14} />
        Key: <span className="font-mono text-text">{provider.keyLast4 ? `•••• ${provider.keyLast4}` : 'saved'}</span>
      </span>
      <p className="m-0 text-[13px] text-text-muted-strong">
        Models: {provider.models.length ? <span className="text-text">{provider.models.join(', ')}</span> : 'none picked yet'}
      </p>
      {remove.error && (
        <p role="alert" className="m-0 text-[13px] text-critical">
          {remove.error.message}
        </p>
      )}
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
    <form onSubmit={submit} noValidate aria-label="Add provider" className="flex flex-col gap-4 border-t border-line-soft pt-5">
      <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-4">
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
      {errors.general && (
        <p role="alert" className="m-0 text-[13px] text-critical">
          {errors.general}
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-2.5">
        <Button onClick={onCancel}>{actionLabels.cancel}</Button>
        <Button type="submit" variant="primary" disabled={create.isPending}>
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
      <p className="m-0">
        <span className="font-medium">{provider.name}</span> is added. Pick the models to use from it.
      </p>
      <ModelPicker available={available} chosen={models} onChange={setModels} error={errors.on('models')} idPrefix={`pick-${provider.id}`} />
      {errors.general && (
        <p role="alert" className="m-0 text-[13px] text-critical">
          {errors.general}
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-2.5">
        <Button onClick={onDone}>Later</Button>
        <Button type="submit" variant="primary" disabled={update.isPending}>
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
      <p className="m-0 font-medium">{provider.name}</p>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-4">
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
        <p className="m-0 text-[13px] text-critical">Couldn't list the models: {listError.message}</p>
      ) : (
        <p className="m-0 text-[13px] text-text-muted">Listing models…</p>
      )}
      {errors.general && (
        <p role="alert" className="m-0 text-[13px] text-critical">
          {errors.general}
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-2.5">
        <Button onClick={onDone}>{actionLabels.cancel}</Button>
        <Button type="submit" variant="primary" disabled={update.isPending}>
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
      <legend className="mb-1 p-0 text-[13px] text-text-soft">Models</legend>
      {all.length === 0 ? (
        <p className="m-0 text-[13px] text-text-muted">The endpoint lists no models.</p>
      ) : (
        <ModelCombobox id={`${idPrefix}-models`} aria-label="Models" multiple options={all} selected={chosen} onChange={onChange} placeholder="Search models" />
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
