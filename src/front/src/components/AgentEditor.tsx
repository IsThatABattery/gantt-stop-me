import { Link } from 'react-router'
import type { AgentSettings, Provider } from '../data/api/types'
import { INSTRUCTIONS_MAX, instructionsLength, type AgentErrors, type AgentField } from '../data/views/agentSettings'
import { Field, Select, Textarea } from './controls'
import { ModelCombobox } from './ModelCombobox'

interface AgentEditorProps {
  /** Prefix of the field ids, `${id}-provider` and so on, so the form can focus them */
  id: string
  name: string
  /** One line on what the agent does */
  description: string
  providers: Provider[]
  value: AgentSettings
  onChange: (value: AgentSettings) => void
  errors?: AgentErrors
  /** It must have a model; otherwise "None" is a choice */
  required?: boolean
  /** Default to a high-reasoning model when a provider is picked */
  deep?: boolean
}

// Names of high-reasoning models across providers; a guess from naming, not from the provider.
const deepModel = /opus|-pro\b|thinking|reason|^o\d/i

/** The model an agent starts with when its provider is picked: the first of the provider's chosen models, or for a deep agent the first high-reasoning one. */
const defaultModel = (models: string[], deep?: boolean) => (deep && models.find((m) => deepModel.test(m))) || models[0] || null

/** One LLM agent's provider, model and extra instructions. */
export function AgentEditor({ id, name, description, providers, value, onChange, errors = {}, required, deep }: AgentEditorProps) {
  const provider = providers.find((p) => p.id === value.providerId)
  const models = provider?.models ?? []
  const length = instructionsLength(value.instructions)
  const fieldId = (f: AgentField) => `${id}-${f}`
  const describedBy = (f: AgentField, ...extra: string[]) =>
    [...extra, errors[f] && `${fieldId(f)}-error`].filter(Boolean).join(' ') || undefined
  const error = (f: AgentField) =>
    errors[f] && (
      <span id={`${fieldId(f)}-error`} className="text-xs text-signal-ink">
        {errors[f]}
      </span>
    )

  return (
    <div role="group" aria-labelledby={`${id}-name`} aria-describedby={`${id}-about`} className="flex flex-col gap-4 border-t border-rule pt-5">
      <div>
        <h3 id={`${id}-name`} className="m-0 text-base font-bold">
          {name}
        </h3>
        <p id={`${id}-about`} className="m-0 mt-1 text-[13px] text-ink-2">
          {description}
        </p>
      </div>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4">
        <Field id={fieldId('provider')} label="Provider">
          <Select
            id={fieldId('provider')}
            value={value.providerId ?? ''}
            disabled={providers.length === 0}
            aria-invalid={!!errors.provider}
            aria-describedby={describedBy('provider')}
            onChange={(e) => {
              const p = providers.find((x) => x.id === Number(e.target.value))
              onChange({ ...value, providerId: p?.id ?? null, model: p ? defaultModel(p.models, deep) : null })
            }}
          >
            <option value="">{providers.length === 0 ? 'No provider added' : required ? 'Choose a provider' : 'None'}</option>
            {providers.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
          {error('provider')}
        </Field>
        <Field id={fieldId('model')} label="Model">
          <ModelCombobox
            id={fieldId('model')}
            options={models}
            selected={value.model ? [value.model] : []}
            onChange={(s) => onChange({ ...value, model: s[0] ?? null })}
            disabled={!provider}
            placeholder={provider ? 'Search models' : 'No provider'}
            quiet
            aria-invalid={!!errors.model}
            aria-describedby={describedBy('model')}
          />
          {value.model && provider && !models.includes(value.model) && <span className="text-xs text-approve-ink">{value.model} is no longer available.</span>}
          {error('model')}
        </Field>
      </div>
      <Field
        id={fieldId('instructions')}
        label={
          <>
            Your instructions <span className="font-normal">(optional)</span>
          </>
        }
      >
        <Textarea
          id={fieldId('instructions')}
          value={value.instructions}
          aria-invalid={!!errors.instructions}
          aria-describedby={describedBy('instructions', `${id}-help`, `${id}-count`)}
          onChange={(e) => onChange({ ...value, instructions: e.target.value })}
        />
        <span className="flex flex-wrap justify-between gap-x-3 text-xs text-ink-2">
          <span id={`${id}-help`}>Added to the agent's built-in instructions. They don't replace them.</span>
          <span id={`${id}-count`} className={length > INSTRUCTIONS_MAX ? 'text-signal-ink' : undefined}>
            {length} / {INSTRUCTIONS_MAX}
          </span>
        </span>
        {error('instructions')}
      </Field>
    </div>
  )
}

/** In place of the editors while the user has no providers to pick from. */
export function NoProviders() {
  return (
    <p className="m-0 text-[13px] text-ink-2">
      You have no LLM providers yet. <Link to="/settings#providers">Add a provider</Link> to choose the agents' models.
    </p>
  )
}
