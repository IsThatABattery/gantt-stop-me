import type { ReactNode } from 'react'
import { Field, Select, Textarea, TextInput } from '../../components/controls'
import type { FormOptions } from '../../data/views/forms'
import { PRIORITIES, type Assignee, type Priority } from '../../data/types'
import { formLabels, labels, priorityLabels, roleLabels } from '../../labels/labels'
import type { BasicsValue, Kind } from './formValue'
import { TagInput } from './TagInput'

const encode = (a: Assignee | null) => (a ? `${a.kind}:${a.id}` : '')
function decode(value: string): Assignee | null {
  const [kind, id] = value.split(':')
  return kind === 'user' || kind === 'agent' ? { kind, id } : null
}

interface BasicsFieldsProps {
  kind: Kind
  value: BasicsValue
  onChange: (next: BasicsValue) => void
  options: FormOptions
  errors?: Partial<Record<'name' | 'estimate', string>>
  /** Dialog: description is optional and the launch window gets its own row */
  variant: 'dialog' | 'page'
  idPrefix: string
}

/** The fields an objective and a payload share (AGENT_BRIEF §7). */
export function BasicsFields({ kind, value, onChange, options, errors = {}, variant, idPrefix }: BasicsFieldsProps) {
  const t = formLabels[kind]
  const id = (name: string) => `${idPrefix}-${name}`
  const set = <K extends keyof BasicsValue>(key: K, v: BasicsValue[K]) => onChange({ ...value, [key]: v })
  // Keep the current parent selectable even if it was archived.
  const parents =
    kind === 'feature'
      ? options.epics
      : options.features.filter((f) => f.status !== 'archived' || f.id === value.parentId).sort((a, b) => a.name.localeCompare(b.name))

  const windowField = (
    <Field id={id('window')} label={formLabels.sprint}>
      <Select id={id('window')} value={value.sprintId ?? ''} onChange={(e) => set('sprintId', e.target.value || null)}>
        {options.sprints.map((s) => (
          <option key={s.id} value={s.id}>
            {labels.sprintName(s.number)}
          </option>
        ))}
        <option value="">{labels.notScheduled}</option>
      </Select>
    </Field>
  )

  return (
    <>
      <Field id={id('name')} label={t.nameLabel}>
        <TextInput
          id={id('name')}
          value={value.name}
          placeholder={t.namePlaceholder}
          onChange={(e) => set('name', e.target.value)}
          aria-invalid={!!errors.name}
          aria-describedby={errors.name ? id('name-error') : undefined}
        />
        <FieldError id={id('name-error')}>{errors.name}</FieldError>
      </Field>

      <Field
        id={id('desc')}
        label={
          variant === 'dialog' ? (
            <>
              {formLabels.description} <span className="text-text-muted">(optional)</span>
            </>
          ) : (
            formLabels.description
          )
        }
      >
        <Textarea
          id={id('desc')}
          rows={3}
          value={value.description}
          placeholder={t.descPlaceholder}
          onChange={(e) => set('description', e.target.value)}
          className="resize-y"
        />
      </Field>

      <div className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4">
        <Field id={id('parent')} label={t.parentLabel}>
          <Select id={id('parent')} value={value.parentId} onChange={(e) => set('parentId', e.target.value)}>
            {parents.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field id={id('people')} label={t.peopleLabel}>
          <Select id={id('people')} value={encode(value.people)} onChange={(e) => set('people', decode(e.target.value))}>
            <option value="">{labels.unassigned}</option>
            {kind === 'task' && (
              <optgroup label={labels.agents}>
                {options.agents.map((a) => (
                  <option key={a.id} value={`agent:${a.id}`}>
                    {a.name} · {labels.agent}
                  </option>
                ))}
              </optgroup>
            )}
            <optgroup label={labels.users}>
              {options.users.map((u) => (
                <option key={u.id} value={`user:${u.id}`}>
                  {u.initials} · {kind === 'feature' ? roleLabels[u.role] : labels.user}
                </option>
              ))}
            </optgroup>
          </Select>
        </Field>
        {variant === 'page' && windowField}
      </div>

      {kind === 'task' && (
        <div className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4">
          <Field id={id('estimate')} label={formLabels.estimate}>
            <TextInput
              id={id('estimate')}
              type="number"
              min={0}
              placeholder="3"
              value={value.estimate}
              onChange={(e) => set('estimate', e.target.value)}
              aria-invalid={!!errors.estimate}
              aria-describedby={errors.estimate ? id('estimate-error') : undefined}
            />
            <FieldError id={id('estimate-error')}>{errors.estimate}</FieldError>
          </Field>
          <Field id={id('priority')} label={formLabels.priority}>
            <Select id={id('priority')} value={value.priority} onChange={(e) => set('priority', e.target.value as Priority)}>
              {PRIORITIES.map((p) => (
                <option key={p} value={p}>
                  {priorityLabels[p]}
                </option>
              ))}
            </Select>
          </Field>
        </div>
      )}

      {variant === 'dialog' && windowField}

      <TagInput
        id={id('tags')}
        label={variant === 'dialog' ? formLabels.tagsOptional : formLabels.tags}
        tags={value.tags}
        onChange={(tags) => set('tags', tags)}
      />
    </>
  )
}

export function FieldError({ id, children }: { id: string; children?: ReactNode }) {
  if (!children) return null
  return (
    <span id={id} className="text-[13px] text-critical">
      {children}
    </span>
  )
}
